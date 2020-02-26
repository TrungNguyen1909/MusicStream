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
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/deezer"
	"github.com/TrungNguyen1909/MusicStream/queue"
	"github.com/TrungNguyen1909/MusicStream/radio"
	"github.com/TrungNguyen1909/MusicStream/vorbisencoder"
	"github.com/gorilla/websocket"
	_ "github.com/joho/godotenv/autoload"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
	"github.com/tdewolff/minify/js"
	mJSON "github.com/tdewolff/minify/json"
	"github.com/tdewolff/minify/svg"
	"github.com/tdewolff/minify/xml"
)

const (
	chunkDelayMS          = 40
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

//#region Global Variables
var upgrader = websocket.Upgrader{}
var connections sync.Map
var currentTrack common.Track
var currentTrackMeta common.TrackMetadata
var dzClient *deezer.Client
var playQueue *queue.Queue
var channels [2]chan chan chunk
var currentChannel int
var oggHeader []byte
var listenersCount int32
var bufferingChannel chan chunk
var etaDone atomic.Value
var skipChannel chan int
var quitRadio chan int
var isRadioStreaming int32
var currentTrackID string
var watchDog int
var radioTrack *radio.Track
var startPos int64
var encoder *vorbisencoder.Encoder
var deltaChannel chan int64
var startTime time.Time
var cacheQueue *queue.Queue
var streamMux sync.Mutex
var encoderWg sync.WaitGroup
var initialized chan int
var minifier *minify.M
var activityWg sync.WaitGroup
var newListenerC chan int

func audioManager() {
	for i := range channels {
		channels[i] = make(chan chan chunk, 1000)
	}
	bufferingChannel = make(chan chunk, 5000)
	skipChannel = make(chan int, 500)
	deltaChannel = make(chan int64, 1)
	quitRadio = make(chan int, 10)
	newListenerC = make(chan int, 1)
	encoder = vorbisencoder.NewEncoder(2, 48000, 256000)
	oggHeader = make([]byte, 5000)
	n := encoder.Encode(oggHeader, make([]byte, 0))
	oggHeader = oggHeader[:n]

	dzClient = deezer.NewClient()
	cacheQueue = queue.NewQueue()
	playQueue = queue.NewQueue()
	radioTrack = radio.NewTrack()
	currentTrackID = ""
	etaDone.Store(time.Now())
	initialized <- 1
	for {
		processTrack()
	}
}
func main() {
	_, ok := os.LookupEnv("DEEZER_ARL")
	if !ok {
		log.Panic("Deezer token not found")
	}
	_, ok = os.LookupEnv("MUSIXMATCH_USER_TOKEN")
	if !ok {
		log.Panic("Musixmatch token not found")
	}
	_, ok = os.LookupEnv("YOUTUBE_DEVELOPER_KEY")
	if !ok {
		log.Panic("Youtube Data API v3 key not found")
	}
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8890"
	}
	port = ":" + port
	initialized = make(chan int)
	go audioManager()
	minifier = minify.New()
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFunc("text/html", html.Minify)
	minifier.AddFunc("image/svg+xml", svg.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("[/+]json$"), mJSON.Minify)
	minifier.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)
	http.HandleFunc("/enqueue", enqueueHandler)
	http.HandleFunc("/queue", queueHandler)
	http.HandleFunc("/listeners", listenersHandler)
	http.HandleFunc("/audio", audioHandler)
	http.HandleFunc("/status", wsHandler)
	http.HandleFunc("/playing", playingHandler)
	http.HandleFunc("/skip", skipHandler)
	http.HandleFunc("/remove", removeTrackHandler)
	http.HandleFunc("/", fileServer(http.Dir("www")))
	go selfPinger()
	<-initialized
	go inactivityMonitor()
	log.Printf("Serving on port %s", port)
	log.Fatal(http.ListenAndServe(port, logRequest(http.DefaultServeMux)))
}
