/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020  Nguyễn Hoàng Trung
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

package main

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
	_ "github.com/joho/godotenv/autoload"
)

func audioHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Fatal("expected http.ResponseWriter to be an http.Flusher")
	}
	atomic.AddInt32(&listenersCount, 1)
	newListenerC <- 1
	go setListenerCount()
	defer setListenerCount()
	defer atomic.AddInt32(&listenersCount, -1)
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "application/ogg")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("status", "200")
	w.Write(oggHeader)
	flusher.Flush()
	channel := make(chan chunk, 500)
	channels[0] <- channel
	chanidx := 0
	for {
		Chunk := <-channel
		chanidx = Chunk.channel
		_, err := w.Write(Chunk.buffer)
		if err != nil {
			break
		}
		flusher.Flush()
		channels[chanidx] <- channel
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	_c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	c := &webSocket{conn: _c, mux: &sync.Mutex{}}
	connections.Store(c, c)
	defer c.Close()
	defer connections.Delete(c)
	for {
		c.WriteMessage(websocket.TextMessage, getPlaying())
		c.WriteMessage(websocket.TextMessage, getQueue())
		var msg wsMessage
		err = c.ReadJSON(&msg)
		if err != nil {
			break
		}
		switch msg.Operation {
		case opSetClientsTrack:
			c.WriteMessage(websocket.TextMessage, getPlaying())
		case opClientRequestTrack:
			c.WriteMessage(websocket.TextMessage, enqueue(msg))
		case opClientRequestSkip:
			c.WriteMessage(websocket.TextMessage, skip())
		case opSetClientsListeners:
			c.WriteMessage(websocket.TextMessage, getListenersCount())
		case opClientRemoveTrack:
			c.WriteMessage(websocket.TextMessage, removeTrack(msg))
		case opClientRequestQueue:
			c.WriteMessage(websocket.TextMessage, getQueue())
		case opWebSocketKeepAlive:
			data, _ := json.Marshal(map[string]interface{}{
				"op": opWebSocketKeepAlive,
			})
			c.WriteMessage(websocket.TextMessage, data)
		}
	}

}

func playingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(getPlaying())
	return
}

func listenersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(getListenersCount())
	return
}

func enqueueHandler(w http.ResponseWriter, r *http.Request) {
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
	w.Write(enqueue(msg))
}
func skipHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(skip())
}
func queueHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(getQueue())
}
func removeTrackHandler(w http.ResponseWriter, r *http.Request) {
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
	w.Write(removeTrack(msg))
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
func fileServer(fs http.Dir) func(w http.ResponseWriter, r *http.Request) {
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
		mContent, err := minifier.Bytes(mediaType, content)
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
