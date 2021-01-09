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
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream"
	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/mp3encoder"
	"github.com/TrungNguyen1909/MusicStream/mxmlyrics"
	"github.com/TrungNguyen1909/MusicStream/queue"
	"github.com/TrungNguyen1909/MusicStream/radio"
	"github.com/TrungNguyen1909/MusicStream/vorbisencoder"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	opListSources         = 1
	opSetClientsTrack     = 2
	opAllClientsSkip      = 3
	opClientRequestTrack  = 4
	opClientRequestSkip   = 5
	opSetClientsListeners = 6
	opTrackEnqueued       = 7
	opClientRequestQueue  = 8
	opWebSocketKeepAlive  = 9
	opClientRemoveTrack   = 10
	opClientAudioStartPos = 11
)

const (
	cookieSessionID = "sessionId"
	defaultStartPos = 0
)

//Server is a MusicStream server
type Server struct {
	upgrader             websocket.Upgrader
	connections          sync.Map
	currentTrack         common.Track
	currentTrackMeta     atomic.Value
	mxmClient            *mxmlyrics.Client
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
	streamContext        context.Context
	skipFunc             context.CancelFunc
	cancelRadio          context.CancelFunc
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
	activityWg           sync.WaitGroup
	newListenerC         chan int
	server               *echo.Echo
	messageHandlers      map[int]RequestHandler
	processedNonce       sync.Map
	authCtxs             sync.Map
	sources              []common.MusicSource
}

//AddMessageHandler registers a new message handler for the specified opcode
func (s *Server) AddMessageHandler(opcode int, handler RequestHandler) {
	s.messageHandlers[opcode] = handler
}

//RemoveMessageHandler unregisters the specified opcode
func (s *Server) RemoveMessageHandler(opcode int) {
	delete(s.messageHandlers, opcode)
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
	if len(MusicStream.BuildVersion) > 0 {
		log.Printf("[MusicStream] MusicStream %s: %s", MusicStream.BuildVersion, MusicStream.BuildTime)
	} else if len(MusicStream.BuildTime) > 0 {
		log.Printf("[MusicStream] MusicStream v%s: %s", MusicStream.Version, MusicStream.BuildTime)
	} else {
		log.Printf("[MusicStream] MusicStream v%s", MusicStream.Version)
	}
	log.Printf("[MusicStream] Starting up at %s", addr)
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
	if len(MusicStream.BuildVersion) > 0 {
		log.Printf("[MusicStream] MusicStream %s: %s", MusicStream.BuildVersion, MusicStream.BuildTime)
	} else {
		log.Printf("[MusicStream] MusicStream v%s", MusicStream.Version)
	}
	log.Printf("[MusicStream] Starting up at %s with auto TLS", addr)
	return s.server.StartAutoTLS(addr)
}

func (s *Server) Shutdown(context context.Context) (err error) {
	err = s.server.Shutdown(context)
	return
}
func (s *Server) Close() error {
	for _, v := range s.sources {
		if closer, ok := v.(io.Closer); ok {
			closer.Close()
		}
	}
	return nil
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
	s.deltaChannel = make(chan int64, 1)
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
	log.Println("[MusicStream] initializing source plugins")
	for _, p := range config.Plugins {
		sName, err := p.Lookup("Name")
		if err != nil {
			log.Println("[MusicStream] Failed to lookup plugin's name")
			continue
		}
		name, ok := sName.(*string)
		if !ok {
			log.Println("[MusicStream] Failed to read plugin's name")
			continue
		}
		sNewClient, err := p.Lookup("NewClient")
		if err != nil {
			log.Printf("[MusicStream] Failed to lookup plugin %s's NewClient", *name)
			continue
		}
		newClient, ok := sNewClient.(func() (common.MusicSource, error))
		if newClient == nil && !ok {
			log.Printf("[MusicStream] Plugin %s's NewClient's signature is incorrect", *name)
			continue
		}
		client, err := newClient()
		if client == nil || err != nil {
			log.Printf("[MusicStream] NewClient failed on plugin %s: %s", *name, err)
			continue
		}
		s.sources = append(s.sources, client)
		log.Printf("[MusicStream] Successfully loaded plugin %s", *name)
	}
	if len(s.sources) <= 0 {
		log.Panic("[MusicStream] ERROR: No sources intialized")
	} else {
		log.Printf("[MusicStream] Loaded %d sources", len(s.sources))
	}
	s.mxmClient, err = mxmlyrics.NewClient(config.MusixMatchUserToken, config.MusixMatchOBUserToken)
	if err != nil {
		log.Println("[MusixMatch] Failed to initalized: ", err)
		err = nil
	}
	s.cacheQueue = queue.New()
	s.playQueue = queue.New()
	s.playQueue.PushCallback = s.enqueueCallback
	s.playQueue.PopCallback = s.dequeueCallback
	if config.RadioEnabled {
		s.radioTrack = radio.NewTrack()
	}
	s.currentTrack = s.defaultTrack
	s.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	s.server = echo.New()
	s.server.Use(middleware.Recover())
	s.server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Cache-Control", "no-cache")
			if _, err := c.Cookie(cookieSessionID); err != nil {
				var sid string
				for {
					sid = common.GenerateID()
					_, exists := s.authCtxs.Load(sid)
					if !exists {
						break
					}
				}
				session := &http.Cookie{
					Name:  cookieSessionID,
					Value: common.GenerateID(),
				}
				c.SetCookie(session)
			}
			return next(c)
		}
	})
	s.server.Use(middleware.Gzip())
	s.server.HTTPErrorHandler = s.HandleError
	s.server.HideBanner = true
	s.messageHandlers = make(map[int]RequestHandler)
	s.AddMessageHandler(opListSources, getSourcesList)
	s.AddMessageHandler(opSetClientsTrack, getPlaying)
	s.AddMessageHandler(opClientRequestTrack, enqueue)
	s.AddMessageHandler(opClientRequestSkip, skip)
	s.AddMessageHandler(opSetClientsListeners, getListenersCount)
	s.AddMessageHandler(opClientRemoveTrack, removeTrack)
	s.AddMessageHandler(opClientRequestQueue, getQueue)
	s.AddMessageHandler(opWebSocketKeepAlive, clientKeepAlivePing)
	s.AddMessageHandler(opClientRemoveTrack, removeTrack)
	s.server.POST("/enqueue", s.enqueueHandler)
	s.server.GET("/listeners", s.listenersHandler)
	s.server.GET("/audio", s.audioHandler)
	s.server.GET("/fallback", s.audioHandler)
	s.server.GET("/status", s.wsHandler)
	s.server.GET("/playing", s.playingHandler)
	s.server.GET("/sources", s.listSourcesHandler)
	s.server.GET("/skip", s.skipHandler)
	s.server.POST("/remove", s.removeTrackHandler)
	s.server.GET("/queue", s.queueHandler)
	if len(config.StaticFilesPath) > 0 {
		s.server.Static("/", config.StaticFilesPath)
	} else {
		s.server.Static("/", "www")
	}
	return s
}
