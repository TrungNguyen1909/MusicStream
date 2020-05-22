/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020 Nguyễn Hoàng Trung(TrungNguyen1909)
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

func (s *Server) audioHandler(c echo.Context) (err error) {
	r := c.Request()
	w := c.Response()
	notify := r.Context().Done()
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("status", "200")
	channel := make(chan *chunk, 500)
	var bufferChannel []chan chan *chunk
	var chanidx int
	isMP3Stream := false
	chunkID := int64(-1)
	if r.URL.Path == "/fallback" {
		w.Header().Set("Content-Type", "audio/mpeg")
		isRanged := len(r.Header.Get("Range")) > 0
		if isRanged {
			w.WriteHeader(200)
			w.Write(s.mp3Header)
			return
		}
		w.Write(s.mp3Header)
		isMP3Stream = true
		bufferChannel = s.mp3Channel
	} else {
		w.Header().Set("Content-Type", "application/ogg")
		w.Write(s.oggHeader)
		bufferChannel = s.vorbisChannel
	}
	if isMP3Stream {
		atomic.AddInt64(s.mp3Subscribers, 1)
		defer atomic.AddInt64(s.mp3Subscribers, -1)
	} else {
		atomic.AddInt64(s.vorbisSubscribers, 1)
		defer atomic.AddInt64(s.vorbisSubscribers, -1)
	}
	firstChunk := true
	defer func() { <-channel }()
	atomic.AddInt32(&s.listenersCount, 1)
	s.newListenerC <- 1
	go s.setListenerCount()
	defer s.setListenerCount()
	defer atomic.AddInt32(&s.listenersCount, -1)
	bufferChannel[0] <- channel
	bufferChannel[1] <- channel
	w.Flush()
	for {
		select {
		case <-notify:
			log.Printf("%s %s %s: client disconnected\n", r.RemoteAddr, r.Method, r.URL)
			return
		case Chunk := <-channel:
			chanidx = Chunk.channel
			if !firstChunk {
				bufferChannel[chanidx] <- channel
			} else {
				firstChunk = false
			}
			if chunkID != -1 && chunkID+1 != Chunk.chunkID {
				log.Println("[", r.URL.Path, "]", "[WARN] chunks from ", chunkID+1, " to ", Chunk.chunkID-1, " have been lost.")
			}
			chunkID = Chunk.chunkID
			_, err := w.Write(Chunk.buffer)
			if err != nil {
				break
			}
		}
	}
}

func (s *Server) wsHandler(c echo.Context) (err error) {
	_c, err := s.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	ws := &webSocket{conn: _c, mux: &sync.Mutex{}}
	s.connections.Store(ws, ws)
	defer ws.Close()
	defer s.connections.Delete(ws)
	ws.WriteMessage(websocket.TextMessage, s.getPlaying())
	ws.WriteMessage(websocket.TextMessage, s.getQueue())
	for {
		var msg wsMessage
		_, msgbuf, err := ws.ReadMessage()
		if err != nil {
			break
		}
		err = json.Unmarshal(msgbuf, &msg)
		if err != nil {
			break
		}
		switch msg.Operation {
		case opSetClientsTrack:
			ws.WriteMessage(websocket.TextMessage, s.getPlaying())
		case opClientRequestTrack:
			ws.WriteMessage(websocket.TextMessage, s.enqueue(msg))
		case opClientRequestSkip:
			ws.WriteMessage(websocket.TextMessage, s.skip())
		case opSetClientsListeners:
			ws.WriteMessage(websocket.TextMessage, s.getListenersCount())
		case opClientRemoveTrack:
			ws.WriteMessage(websocket.TextMessage, s.removeTrack(msg))
		case opClientRequestQueue:
			ws.WriteMessage(websocket.TextMessage, s.getQueue())
		case opWebSocketKeepAlive:
			data, _ := json.Marshal(map[string]interface{}{
				"op": opWebSocketKeepAlive,
			})
			ws.WriteMessage(websocket.TextMessage, data)
		}
	}
	return
}

func (s *Server) playingHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.getPlaying())
	return
}

func (s *Server) listenersHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.getListenersCount())
	return
}

func (s *Server) enqueueHandler(c echo.Context) (err error) {
	r := c.Request()
	var msg wsMessage
	err = json.NewDecoder(r.Body).Decode(&msg)
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": false,
			"reason":  "Invalid Query!",
		})
		w.Write(data)
		return
	}
	w.Write(s.enqueue(msg))
	return
}
func (s *Server) skipHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.skip())
	return
}
func (s *Server) queueHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.getQueue())
	return
}
func (s *Server) removeTrackHandler(c echo.Context) (err error) {
	r := c.Request()
	w := c.Response()
	var msg wsMessage
	err = json.NewDecoder(r.Body).Decode(&msg)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRemoveTrack,
			"success": false,
			"reason":  "Bad Request",
		})
		w.Write(data)
		return
	}
	w.Write(s.removeTrack(msg))
	return
}
func redirectToRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func toHTTPError(err error) (msg string, httpStatus int) {
	if os.IsNotExist(err) {
		return "404 page not found", http.StatusNotFound
	}
	if os.IsPermission(err) {
		return "403 Forbidden", http.StatusForbidden
	}
	// Default:
	return "500 Internal Server Error", http.StatusInternalServerError
}

// NotFoundHandler handles the "not found" situation. It should be a catch-all for all urls.
func NotFoundHandler(c echo.Context) error {
	return echo.NewHTTPError(http.StatusNotFound, "The page you are looking for does not exist")
}
