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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
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
	w.Header().Set("Accept-Ranges", "none")
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
			_, _ = w.Write(s.mp3Header)
			return
		}
		_, _ = w.Write(s.mp3Header)
		isMP3Stream = true
		bufferChannel = s.mp3Channel
	} else {
		w.Header().Set("Content-Type", "audio/ogg")
		isRanged := len(r.Header.Get("Range")) > 0
		if isRanged {
			Range := r.Header.Get("Range")
			var start int
			fmt.Sscanf(Range, "bytes=%d-", start)
			if start != 0 {
				w.WriteHeader(200)
				_, _ = w.Write(s.oggHeader)
				return
			}
		}
		_, _ = w.Write(s.oggHeader)
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
	for err == nil {
		select {
		case <-notify:
			return
		case Chunk := <-channel:
			chanidx = Chunk.channel
			if !firstChunk {
				bufferChannel[chanidx] <- channel
			} else {
				firstChunk = false
				if cookie, err := c.Cookie(cookieSessionID); err == nil && len(cookie.Value) > 0 {
					var ctx *authenticatedContext
					ctx_, _ := s.authCtxs.LoadOrStore(cookie.Value, newAuthenticatedContext())
					ctx = ctx_.(*authenticatedContext)
					if ctx.WS != nil {
						ctx.WS.WriteMessage(websocket.TextMessage, Response{
							Operation: opClientAudioStartPos,
							Success:   true,
							Data: map[string]interface{}{
								"startPos": Chunk.encoderPos,
							},
						}.EncodeJSON())
					}
					ctx.StartPos = Chunk.encoderPos
					s.authCtxs.Store(cookie.Value, ctx)
					defer func() { ctx.StartPos = defaultStartPos }()
				}
			}
			if chunkID != -1 && chunkID+1 != Chunk.chunkID {
				log.Println("[", r.URL.Path, "]", "[WARN] chunks from ", chunkID+1, " to ", Chunk.chunkID-1, " have been lost.")
			}
			chunkID = Chunk.chunkID
			_, err = w.Write(Chunk.buffer)
		}
	}
	return
}
func (s *Server) processNonce(nonce int) {
	s.processedNonce.Store(nonce, nil)
}
func (s *Server) checkNonce(nonce int) bool {
	_, ok := s.processedNonce.Load(nonce)
	return ok
}
func (s *Server) handleMessage(msg *wsMessage) (r []byte) {
	if msg.Nonce != 0 && s.checkNonce(msg.Nonce) {
		return nil
	} else if msg.Nonce != 0 {
		s.processNonce(msg.Nonce)
	}
	if handler, ok := s.messageHandlers[msg.Operation]; ok {
		resp := handler(s, *msg)
		resp.Nonce = msg.Nonce
		return resp.EncodeJSON()
	} else {
		return Response{
			Operation: msg.Operation,
			Nonce:     msg.Nonce,
			Success:   false,
			Reason:    "Invalid opcode",
		}.EncodeJSON()
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
	_ = ws.WriteMessage(websocket.TextMessage, getPlaying(s, wsMessage{}).EncodeJSON())
	_ = ws.WriteMessage(websocket.TextMessage, getQueue(s, wsMessage{}).EncodeJSON())
	if cookie, err := c.Cookie(cookieSessionID); err == nil && len(cookie.Value) > 0 {
		ctx_, _ := s.authCtxs.LoadOrStore(cookie.Value, newAuthenticatedContext())
		ctx := ctx_.(*authenticatedContext)
		ws.WriteMessage(websocket.TextMessage, Response{
			Operation: opClientAudioStartPos,
			Success:   true,
			Data: map[string]interface{}{
				"startPos": ctx.StartPos,
			},
		}.EncodeJSON())
		ctx.WS = ws
		defer func() { ctx.WS = nil }()
	}
	for err == nil {
		var msg wsMessage
		var msgbuf []byte
		_, msgbuf, err = ws.ReadMessage()
		if err != nil {
			break
		}
		err = json.Unmarshal(msgbuf, &msg)
		if err != nil {
			break
		}
		err = ws.WriteMessage(websocket.TextMessage, s.handleMessage(&msg))
	}
	if !websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
		err = nil
	} else {
		err = errors.WithStack(err)
	}
	return
}

func (s *Server) playingHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(s.handleMessage(&wsMessage{Operation: opSetClientsTrack}))
	return
}

func (s *Server) listenersHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(s.handleMessage(&wsMessage{Operation: opSetClientsListeners}))
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
		return echo.NewHTTPError(http.StatusBadRequest, Response{
			Operation: opClientRequestTrack,
			Success:   false,
			Reason:    "Invalid Query!",
		})
	}
	_, _ = w.Write(s.handleMessage(&msg))
	return
}
func (s *Server) skipHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(s.handleMessage(&wsMessage{Operation: opClientRequestSkip}))
	return
}
func (s *Server) queueHandler(c echo.Context) (err error) {
	w := c.Response()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(s.handleMessage(&wsMessage{Operation: opClientRequestQueue}))
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
		return echo.NewHTTPError(http.StatusBadRequest, Response{
			Operation: opClientRemoveTrack,
			Success:   false,
			Reason:    "Bad Request",
		})
	}
	_, _ = w.Write(s.handleMessage(&msg))
	return
}

// HandleError defines an error handler that complies with echo's standards.
func (s *Server) HandleError(err error, c echo.Context) {
	type errCtx struct {
		Code       int
		Message    string
		StatusText string
	}
	// the convention is:
	// - if err is *echo.HTTPError, it is a "normal error" with its own message and everything.
	// - otherwise, it is an unexpected error.

	if e, ok := err.(*echo.HTTPError); ok {
		// Just handle it gracefully
		_ = c.JSON(e.Code, e.Message)
	} else {
		// internal error: dump it.
		_ = c.JSON(http.StatusInternalServerError, errCtx{Code: http.StatusInternalServerError})

		errStr := fmt.Sprintf("An unexpected error has occured: %v\n", err)
		path := filepath.Join(os.TempDir(), fmt.Sprintf("MusicStream-%v.txt", time.Now().Format(time.RFC3339)))
		if err := ioutil.WriteFile(path, []byte(fmt.Sprintf("%+v", err)), 0644); err != nil {
			errStr += fmt.Sprintf("Cannot log the error down to file: %v", err)
		} else {
			errStr += fmt.Sprintf(`The error has been logged down to file '%s'.
Please check out the open issues and help opening a new one if possible on https://github.com/TrungNguyen1909/MusicStream/issues/new`, path)
		}
		log.Println(errStr)
		if s.server.Debug {
			log.Printf("Error dump:\n%+v\n", err)
		}
	}
}
