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
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

func (s *Server) audioHandler(w http.ResponseWriter, r *http.Request) {
	notify := r.Context().Done()
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("expected http.ResponseWriter to be an http.Flusher")
		return
	}
	atomic.AddInt32(&s.listenersCount, 1)
	s.newListenerC <- 1
	go s.setListenerCount()
	defer s.setListenerCount()
	defer atomic.AddInt32(&s.listenersCount, -1)
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
	bufferChannel[chanidx] <- channel
	flusher.Flush()
	for {
		select {
		case <-notify:
			log.Printf("%s %s %s: client disconnected\n", r.RemoteAddr, r.Method, r.URL)
			<-channel
			return
		case Chunk := <-channel:
			chanidx = Chunk.channel
			bufferChannel[chanidx] <- channel
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

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	s.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	_c, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	c := &webSocket{conn: _c, mux: &sync.Mutex{}}
	s.connections.Store(c, c)
	defer c.Close()
	defer s.connections.Delete(c)
	c.WriteMessage(websocket.TextMessage, s.getPlaying())
	c.WriteMessage(websocket.TextMessage, s.getQueue())
	for {
		var msg wsMessage
		err = c.ReadJSON(&msg)
		if err != nil {
			break
		}
		switch msg.Operation {
		case opSetClientsTrack:
			c.WriteMessage(websocket.TextMessage, s.getPlaying())
		case opClientRequestTrack:
			c.WriteMessage(websocket.TextMessage, s.enqueue(msg))
		case opClientRequestSkip:
			c.WriteMessage(websocket.TextMessage, s.skip())
		case opSetClientsListeners:
			c.WriteMessage(websocket.TextMessage, s.getListenersCount())
		case opClientRemoveTrack:
			c.WriteMessage(websocket.TextMessage, s.removeTrack(msg))
		case opClientRequestQueue:
			c.WriteMessage(websocket.TextMessage, s.getQueue())
		case opWebSocketKeepAlive:
			data, _ := json.Marshal(map[string]interface{}{
				"op": opWebSocketKeepAlive,
			})
			c.WriteMessage(websocket.TextMessage, data)
		}
	}

}

func (s *Server) playingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.getPlaying())
	return
}

func (s *Server) listenersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.getListenersCount())
	return
}

func (s *Server) enqueueHandler(w http.ResponseWriter, r *http.Request) {
	var msg wsMessage
	err := json.NewDecoder(r.Body).Decode(&msg)
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
}
func (s *Server) skipHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.skip())
}
func (s *Server) queueHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(s.getQueue())
}
func (s *Server) removeTrackHandler(w http.ResponseWriter, r *http.Request) {
	var msg wsMessage
	err := json.NewDecoder(r.Body).Decode(&msg)
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
func (s *Server) fileServer(fs http.Dir) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			r.URL.Path = upath
		}
		name := path.Clean(upath)
		const indexPage = "/index.html"

		// redirect .../index.html to .../
		// can't use Redirect() because that would make the path absolute,
		// which would be a problem running under StripPrefix
		if strings.HasSuffix(r.URL.Path, indexPage) {
			http.Redirect(w, r, "./", http.StatusSeeOther)
			return
		}

		f, err := fs.Open(name)
		if err != nil {
			msg, code := toHTTPError(err)
			http.Error(w, msg, code)
			return
		}
		defer f.Close()

		d, err := f.Stat()
		if err != nil {
			msg, code := toHTTPError(err)
			http.Error(w, msg, code)
			return
		}

		// redirect to canonical path: / at end of directory url
		// r.URL.Path always begins with /
		url := r.URL.Path
		if d.IsDir() {
			if url[len(url)-1] != '/' {
				http.Redirect(w, r, path.Base(url)+"/", http.StatusSeeOther)
				return
			}
		} else {
			if url[len(url)-1] == '/' {
				http.Redirect(w, r, "../"+path.Base(url), http.StatusSeeOther)
				return
			}
		}

		// redirect if the directory name doesn't end in a slash
		if d.IsDir() {
			url := r.URL.Path
			if url[len(url)-1] != '/' {
				http.Redirect(w, r, path.Base(url)+"/", http.StatusSeeOther)
				return
			}
		}

		// use contents of index.html for directory, if present
		if d.IsDir() {
			index := strings.TrimSuffix(name, "/") + indexPage
			ff, err := fs.Open(index)
			if err == nil {
				defer ff.Close()
				dd, err := ff.Stat()
				if err == nil {
					name = index
					d = dd
					f = ff
				}
			}
		}

		// Still a directory? (we didn't find an index.html file)
		if d.IsDir() {
			msg, code := toHTTPError(os.ErrPermission)
			http.Error(w, msg, code)
			return
		}

		// serveContent will check modification time
		content, err := ioutil.ReadAll(f)
		if err != nil {
			msg, code := toHTTPError(err)
			http.Error(w, msg, code)
			return
		}
		mediaType := mime.TypeByExtension(filepath.Ext(d.Name()))
		if mediaType == "" {
			mediaType = http.DetectContentType(content)
		}
		mContent, err := s.minifier.Bytes(mediaType, content)
		etag := sha1.Sum(mContent)
		w.Header().Set("ETag", "W/"+fmt.Sprintf("%x", etag))
		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, w.Header().Get("ETag")) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		http.ServeContent(w, r, d.Name(), d.ModTime(), bytes.NewReader(mContent))
		return

	}
}
func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		handler.ServeHTTP(w, r)
	})
}
