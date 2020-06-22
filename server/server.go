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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/TrungNguyen1909/MusicStream"
	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/csn"
	"github.com/TrungNguyen1909/MusicStream/deezer"
	"github.com/TrungNguyen1909/MusicStream/mp3encoder"
	"github.com/TrungNguyen1909/MusicStream/mxmlyrics"
	"github.com/TrungNguyen1909/MusicStream/queue"
	"github.com/TrungNguyen1909/MusicStream/radio"
	"github.com/TrungNguyen1909/MusicStream/vorbisencoder"
	"github.com/TrungNguyen1909/MusicStream/youtube"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	upgrader             websocket.Upgrader
	connections          sync.Map
	currentTrack         common.Track
	currentTrackMeta     common.TrackMetadata
	dzClient             *deezer.Client
	ytClient             *youtube.Client
	mxmClient            *mxmlyrics.Client
	csnClient            *csn.Client
	playQueue            *queue.Queue
	currentVorbisChannel int
	currentMP3Channel    int
	vorbisSubscribers    *int64
	mp3Subscribers       *int64
	vorbisChannel        []chan chan *chunk
	mp3Channel           []chan chan *chunk
	oggHeader            []byte
	mp3Header            []byte
	vorbisChunkID        *int64
	mp3ChunkID           *int64
	listenersCount       int32
	bufferingChannel     chan *chunk
	skipChannel          chan int
	quitRadio            chan int
	isRadioStreaming     int32
	currentTrackID       string
	watchDog             int
	radioTrack           *radio.Track
	defaultTrack         *common.DefaultTrack
	startPos             int64
	lastStreamEnded      time.Time
	vorbisEncoder        *vorbisencoder.Encoder
	mp3Encoder           *mp3encoder.Encoder
	deltaChannel         chan int64
	startTime            time.Time
	cacheQueue           *queue.Queue
	streamMux            sync.Mutex
	encoderWg            sync.WaitGroup
	minifier             *minify.M
	activityWg           sync.WaitGroup
	newListenerC         chan int
	server               *echo.Echo
}

//Start starts the server, listening at addr
func (s *Server) Start(addr string) (err error) {
	go s.selfPinger()
	go s.inactivityMonitor()
	go func() {
		for {
			s.processTrack()
		}
	}()
	log.Printf("Starting MusicStream v%s at %s", MusicStream.Version, addr)
	return s.server.Start(addr)
}

//StartWithTLS starts the server, listening at addr, also tries to get a cert from LetsEncrypt
func (s *Server) StartWithTLS(addr string) (err error) {
	go s.selfPinger()
	go s.inactivityMonitor()
	go func() {
		for {
			s.processTrack()
		}
	}()
	log.Printf("Starting MusicStream v%s at %s with TLS", MusicStream.Version, addr)
	return s.server.StartAutoTLS(addr)
}

//NewServer returns a new server
func NewServer(config Config) *Server {
	s := &Server{}
	s.bufferingChannel = make(chan *chunk, 5000)
	s.mp3Channel = make([]chan chan *chunk, 2)
	s.vorbisChannel = make([]chan chan *chunk, 2)
	for i := 0; i < 2; i++ {
		s.mp3Channel[i] = make(chan chan *chunk, 500)
		s.vorbisChannel[i] = make(chan chan *chunk, 500)
	}
	s.vorbisSubscribers = new(int64)
	s.mp3Subscribers = new(int64)
	s.vorbisChunkID = new(int64)
	s.mp3ChunkID = new(int64)
	s.skipChannel = make(chan int, 500)
	s.deltaChannel = make(chan int64, 1)
	s.quitRadio = make(chan int, 10)
	s.newListenerC = make(chan int, 1)
	s.vorbisEncoder = vorbisencoder.NewEncoder(2, 48000, 320000)
	s.oggHeader = make([]byte, 5000)
	n := s.vorbisEncoder.Encode(s.oggHeader, make([]byte, 0))
	s.oggHeader = s.oggHeader[:n]
	s.mp3Encoder = mp3encoder.NewEncoder(2, 48000, 320000)
	s.mp3Header = make([]byte, 8000)
	n = s.mp3Encoder.Encode(s.mp3Header, make([]byte, 1152*4))
	s.mp3Header = s.mp3Header[:n]

	var err error
	s.dzClient, err = deezer.NewClient(config.DeezerARL, config.SpotifyClientID, config.SpotifyClientSecret)
	if err != nil {
		log.Println("[DZ] Failed to initalized source:", err)
		err = nil
	}
	s.csnClient, err = csn.NewClient()
	if err != nil {
		log.Println("[CSN] Failed to initalized source:", err)
		err = nil
	}
	s.ytClient, err = youtube.NewClient(config.YoutubeDeveloperKey)
	if err != nil {
		log.Println("[YT] Failed to initalized source:", err)
		err = nil
	}
	s.mxmClient, err = mxmlyrics.NewClient(config.MusixMatchUserToken, config.MusixMatchOBUserToken)
	if err != nil {
		log.Println("[MXM] Failed to initalized source:", err)
		err = nil
	}
	s.cacheQueue = queue.NewQueue()
	s.playQueue = queue.NewQueue()
	s.playQueue.EnqueueCallback = s.enqueueCallback
	s.playQueue.DequeueCallback = s.dequeueCallback
	if config.RadioEnabled {
		s.radioTrack = radio.NewTrack()
	}
	s.currentTrack = s.defaultTrack
	s.currentTrackID = ""
	s.minifier = minify.New()
	s.minifier.AddFunc("text/css", css.Minify)
	s.minifier.AddFunc("text/html", html.Minify)
	s.minifier.AddFunc("image/svg+xml", svg.Minify)
	s.minifier.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	s.minifier.AddFuncRegexp(regexp.MustCompile("[/+]json$"), mJSON.Minify)
	s.minifier.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)
	s.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	s.server = echo.New()
	s.server.Use(middleware.Recover())
	s.server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Cache-Control", "no-cache")
			return next(c)
		}
	})
	s.server.Use(middleware.Gzip())
	s.server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			res := c.Response()
			r := c.Request()
			if c.IsWebSocket() || strings.HasPrefix(r.URL.Path, "/audio") || strings.HasPrefix(r.URL.Path, "/fallback") {
				return next(c)
			}
			mw := s.minifier.ResponseWriter(res.Writer, c.Request())
			defer mw.Close()
			res.Writer = mw
			return next(c)
		}
	})

	s.server.HideBanner = true
	s.server.POST("/enqueue", s.enqueueHandler)
	s.server.GET("/listeners", s.listenersHandler)
	s.server.GET("/audio", s.audioHandler)
	s.server.GET("/fallback", s.audioHandler)
	s.server.GET("/status", s.wsHandler)
	s.server.GET("/playing", s.playingHandler)
	s.server.GET("/skip", s.skipHandler)
	s.server.POST("/remove", s.removeTrackHandler)
	s.server.GET("/queue", s.queueHandler)
	s.server.Static("/", "www")
	return s
}
