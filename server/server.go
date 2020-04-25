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
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream"
	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/deezer"
	"github.com/TrungNguyen1909/MusicStream/queue"
	"github.com/TrungNguyen1909/MusicStream/radio"
	"github.com/TrungNguyen1909/MusicStream/vorbisencoder"
	"github.com/gorilla/websocket"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
	"github.com/tdewolff/minify/js"
	mJSON "github.com/tdewolff/minify/json"
	"github.com/tdewolff/minify/svg"
	"github.com/tdewolff/minify/xml"
)

const (
	chunkDelayMS          = 50
	opSetClientsTrack     = 1
	opAllClientsSkip      = 2
	opClientRequestTrack  = 3
	opClientRequestSkip   = 4
	opSetClientsListeners = 5
	opTrackEnqueued       = 6
	opClientRequestQueue  = 7
	opWebSocketKeepAlive  = 8
	opClientRemoveTrack   = 9
)

//Server is a MusicStream server
type Server struct {
	upgrader         websocket.Upgrader
	connections      sync.Map
	currentTrack     common.Track
	currentTrackMeta common.TrackMetadata
	dzClient         *deezer.Client
	playQueue        *queue.Queue
	channels         [2]chan chan chunk
	currentChannel   int
	oggHeader        []byte
	listenersCount   int32
	bufferingChannel chan chunk
	etaDone          atomic.Value
	skipChannel      chan int
	quitRadio        chan int
	isRadioStreaming int32
	currentTrackID   string
	watchDog         int
	radioTrack       *radio.Track
	defaultTrack     *common.DefaultTrack
	startPos         int64
	encoder          *vorbisencoder.Encoder
	deltaChannel     chan int64
	startTime        time.Time
	cacheQueue       *queue.Queue
	streamMux        sync.Mutex
	encoderWg        sync.WaitGroup
	minifier         *minify.M
	activityWg       sync.WaitGroup
	newListenerC     chan int
	serveMux         *http.ServeMux
	server           *http.Server
}

//Serve starts the server, listening at addr
func (s *Server) Serve(addr string) (err error) {
	go s.selfPinger()
	go s.inactivityMonitor()
	go func() {
		for {
			s.processTrack()
		}
	}()
	log.Printf("Starting MusicStream v%s at %s", MusicStream.Version, addr)
	s.server = &http.Server{Addr: addr, Handler: logRequest(s.serveMux)}
	return s.server.ListenAndServe()
}

//NewServer returns a new server
func NewServer() *Server {
	s := &Server{}
	for i := range s.channels {
		s.channels[i] = make(chan chan chunk, 1000)
	}
	s.bufferingChannel = make(chan chunk, 5000)
	s.skipChannel = make(chan int, 500)
	s.deltaChannel = make(chan int64, 1)
	s.quitRadio = make(chan int, 10)
	s.newListenerC = make(chan int, 1)
	s.encoder = vorbisencoder.NewEncoder(2, 48000, 0.9)
	s.oggHeader = make([]byte, 5000)
	n := s.encoder.Encode(s.oggHeader, make([]byte, 0))
	s.oggHeader = s.oggHeader[:n]

	s.dzClient = deezer.NewClient()
	s.cacheQueue = queue.NewQueue()
	s.playQueue = queue.NewQueue()
	if radioDisabled, ok := os.LookupEnv("RADIO_DISABLED"); !ok && len(radioDisabled) > 0 {
		s.radioTrack = radio.NewTrack()
	}
	s.currentTrack = s.defaultTrack
	s.currentTrackID = ""
	s.etaDone.Store(time.Now())
	s.minifier = minify.New()
	s.minifier.AddFunc("text/css", css.Minify)
	s.minifier.AddFunc("text/html", html.Minify)
	s.minifier.AddFunc("image/svg+xml", svg.Minify)
	s.minifier.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	s.minifier.AddFuncRegexp(regexp.MustCompile("[/+]json$"), mJSON.Minify)
	s.minifier.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)
	s.serveMux = http.NewServeMux()
	s.serveMux.HandleFunc("/enqueue", s.enqueueHandler)
	s.serveMux.HandleFunc("/queue", s.queueHandler)
	s.serveMux.HandleFunc("/listeners", s.listenersHandler)
	s.serveMux.HandleFunc("/audio", s.audioHandler)
	s.serveMux.HandleFunc("/status", s.wsHandler)
	s.serveMux.HandleFunc("/playing", s.playingHandler)
	s.serveMux.HandleFunc("/skip", s.skipHandler)
	s.serveMux.HandleFunc("/remove", s.removeTrackHandler)
	s.serveMux.HandleFunc("/", s.fileServer(http.Dir("www")))
	return s
}