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
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/csn"
	"github.com/TrungNguyen1909/MusicStream/deezer"
	"github.com/TrungNguyen1909/MusicStream/lyrics"
	"github.com/TrungNguyen1909/MusicStream/queue"
	"github.com/TrungNguyen1909/MusicStream/radio"
	"github.com/TrungNguyen1909/MusicStream/vorbisencoder"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
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

//#region Structures

type chunk struct {
	buffer      []byte
	encoderTime time.Duration
	channel     int
}
type wsMessage struct {
	Operation int    `json:"op"`
	Query     string `json:"query"`
	Selector  int    `json:"selector"`
}

//#region webSocket

type webSocket struct {
	conn *websocket.Conn
	mux  *sync.Mutex
}

func (socket *webSocket) WriteMessage(messageType int, data []byte) error {
	socket.mux.Lock()
	defer socket.mux.Unlock()
	return socket.conn.WriteMessage(messageType, data)
}
func (socket *webSocket) Close() error {
	socket.mux.Lock()
	defer socket.mux.Unlock()
	return socket.conn.Close()
}
func (socket *webSocket) ReadJSON(v interface{}) error {
	return socket.conn.ReadJSON(v)
}

//#endregion

//#endregion

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
var currentTrackID int
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

//#endregion

//#region Radio Stream

func encodeRadio(stream io.ReadCloser, encodedTime *time.Duration, quit chan int) (ended bool) {
	streamer, format, err := vorbis.Decode(stream)
	if err != nil {
		log.Println(err)
		return false
	}

	defer streamer.Close()

	samples := make([][2]float64, 960)
	buf := make([]byte, len(samples)*format.Width())
	for {
		select {
		case <-quit:
			return true
		default:
		}
		n, ok := streamer.Stream(samples)
		if !ok {
			return false
		}
		for i, sample := range samples {
			switch {
			case format.Precision == 1:
				format.EncodeUnsigned(buf[i*format.Width():], sample)
			case format.Precision == 2 || format.Precision == 3:
				format.EncodeSigned(buf[i*format.Width():], sample)
			default:
				panic(fmt.Errorf("encode: invalid precision: %d", format.Precision))
			}
		}
		pushPCMAudio(buf[:n*format.Width()], encodedTime)
		if 0 <= n && n < len(samples) && ok {
			return false
		}
	}
}
func preloadRadio(quit chan int) {
	var encodedTime time.Duration
	time.Sleep(time.Until(etaDone.Load().(time.Time)))
	log.Println("Radio preloading started!")
	defer endCurrentStream()
	defer pushSilentFrames(&encodedTime)
	defer log.Println("Radio preloading stopped!")
	quitRadioSetTrack := make(chan int, 1)
	go func(quit chan int) {
		firstTime := true
		for {
			if !firstTime {
				radioTrack.WaitForTrackUpdate()
			} else {
				firstTime = false
			}
			select {
			case <-quitRadioSetTrack:
				return
			default:
			}
			pos := int64(encoder.GranulePos())
			atomic.StoreInt64(&startPos, pos)
			deltaChannel <- pos
			setTrack(common.GetMetadata(radioTrack))
		}
	}(quitRadioSetTrack)
	stream, _ := radioTrack.Download()
	for !encodeRadio(stream, &encodedTime, quit) {
		stream, _ = radioTrack.Download()
	}
	quitRadioSetTrack <- 1
}
func processRadio(quit chan int) {
	quitPreload := make(chan int, 10)
	radioTrack.InitWS()
	time.Sleep(time.Until(etaDone.Load().(time.Time)))
	currentTrack = radioTrack
	go preloadRadio(quitPreload)
	atomic.StoreInt32(&isRadioStreaming, 1)
	defer atomic.StoreInt32(&isRadioStreaming, 0)
	defer log.Println("Radio stream ended")
	defer radioTrack.CloseWS()
	defer func() { log.Println("Resuming track streaming..."); quit <- 0 }()
	streamToClients(quit, quitPreload)
}

//#endregion

//#region Track Stream

func preloadTrack(stream io.ReadCloser, quit chan int) {
	var encodedTime time.Duration
	streamer, format, err := mp3.Decode(stream)
	if err != nil {
		log.Panic(err)
	}
	defer streamer.Close()
	var needResampling bool
	var resampled *beep.Resampler
	if format.SampleRate != beep.SampleRate(48000) {
		resampled = beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
		format.SampleRate = beep.SampleRate(48000)
		needResampling = true
	}
	defer endCurrentStream()
	pushSilentFrames(&encodedTime)
	defer pushSilentFrames(&encodedTime)
	pos := int64(encoder.GranulePos())
	atomic.StoreInt64(&startPos, pos)
	deltaChannel <- pos

	samples := make([][2]float64, 960)
	buf := make([]byte, len(samples)*format.Width())
	for {
		select {
		case <-quit:
			return
		default:
		}
		var (
			n  int
			ok bool
		)
		if needResampling {
			n, ok = resampled.Stream(samples)
		} else {
			n, ok = streamer.Stream(samples)
		}
		if !ok {
			break
		}
		for i, sample := range samples {
			switch {
			case format.Precision == 1:
				format.EncodeUnsigned(buf[i*format.Width():], sample)
			case format.Precision == 2 || format.Precision == 3:
				format.EncodeSigned(buf[i*format.Width():], sample)
			default:
				panic(fmt.Errorf("encode: invalid precision: %d", format.Precision))
			}
		}
		pushPCMAudio(buf[:n*format.Width()], &encodedTime)
		if 0 <= n && n < len(samples) && ok {
			return
		}
	}
}
func processTrack() {
	defer func() {
		if r := recover(); r != nil {
			watchDog++
			log.Println("Panicked!!!:", r)
			if currentTrack.Source() == common.Deezer {
				log.Println("Creating a new deezer client...")
				dzClient = deezer.NewClient()
			}
			log.Println("Resuming...")
		}
	}()
	var track common.Track
	var err error
	radioStarted := false
	if currentTrackID == -1 || watchDog >= 3 || currentTrack.Source() == common.CSN {
		if playQueue.Empty() {
			radioStarted = true
			go processRadio(quitRadio)
		}
		activityWg.Wait()
		track = playQueue.Pop().(common.Track)
		dequeueCallback()
		currentTrackID = -1
		watchDog = 0
	} else {
		err = dzClient.PopulateMetadata(currentTrack.(*deezer.Track))
		track = currentTrack
		if err != nil {
			currentTrackID = -1
			watchDog = 0
			return
		}
	}
	activityWg.Wait()
	currentTrackID = track.ID()
	currentTrack = track
	if radioStarted {
		quitRadio <- 0
	}
	if track.Source() == common.CSN {
		cTrack := track.(csn.Track)
		err = cTrack.Populate()
		if err != nil {
			log.Panic(err)
		}
		track = cTrack
	}
	log.Printf("Playing %v - %v\n", track.Title(), track.Artist())
	trackDict := common.GetMetadata(track)
	var mxmlyrics common.LyricsResult
	mxmlyrics, err = lyrics.GetLyrics(track.Title(), track.Artist(), track.Album(), track.Artists(), track.SpotifyURI(), track.Duration())
	if err == nil {
		trackDict.Lyrics = mxmlyrics
	}
	stream, err := track.Download()
	if err != nil {
		log.Panic(err)
	}
	quit := make(chan int, 10)
	if radioStarted {
		<-quitRadio
	}
	go preloadTrack(stream, quit)
	for len(skipChannel) > 0 {
		select {
		case <-skipChannel:
		default:
		}
	}
	time.Sleep(time.Until(etaDone.Load().(time.Time)))
	startTime = time.Now()
	setTrack(trackDict)
	streamToClients(skipChannel, quit)
	log.Println("Stream ended!")
	currentTrackID = -1
	watchDog = 0
}

//#endregion

//#region Data Distribution
func pushPCMAudio(pcm []byte, encodedTime *time.Duration) {
	output := make([]byte, 20000)
	encodedLen := encoder.Encode(output, pcm)
	output = output[:encodedLen]
	*encodedTime += (time.Duration)(len(pcm)/4/48) * time.Millisecond
	if len(output) > 0 {
		bufferingChannel <- chunk{buffer: output, encoderTime: *encodedTime}
	}
}
func pushSilentFrames(encodedTime *time.Duration) {
	silenceBuffer := make([]byte, 76032)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			pushPCMAudio(silenceBuffer, encodedTime)
		}
	}
}
func endCurrentStream() {
	bufferingChannel <- chunk{buffer: nil, encoderTime: 0}
}
func streamToClients(quit chan int, quitPreload chan int) {
	streamMux.Lock()
	defer streamMux.Unlock()
	start := time.Now()
	etaDone.Store(start)
	interrupted := false
	for {
		select {
		case <-quit:
			quitPreload <- 0
			interrupted = true
			for len(quit) > 0 {
				select {
				case <-quit:
				default:
				}
			}
		default:
		}
		if !interrupted {
			Chunk := <-bufferingChannel
			if Chunk.buffer == nil {
				log.Println("Found last chunk, breaking...")
				break
			}
			done := false
			Chunk.channel = ((currentChannel + 1) % 2)
			for !done {
				select {
				case c := <-channels[currentChannel]:
					select {
					case c <- Chunk:
					default:
					}
				default:
					currentChannel = (currentChannel + 1) % 2
					done = true
				}
			}
			etaDone.Store(start.Add(Chunk.encoderTime))
			time.Sleep(Chunk.encoderTime - time.Since(start) - chunkDelayMS*time.Millisecond)
		} else {
			for {
				Chunk := <-bufferingChannel
				if Chunk.buffer == nil {
					log.Println("Found last chunk, breaking...")
					break
				}
			}
			return
		}
	}
}
func setTrack(trackMeta common.TrackMetadata) {
	currentTrackMeta = trackMeta
	log.Printf("Setting track on all clients %v - %v\n", trackMeta.Title, trackMeta.Artist)
	data, err := json.Marshal(map[string]interface{}{
		"op":        opSetClientsTrack,
		"track":     trackMeta,
		"pos":       <-deltaChannel,
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	connections.Range(func(key, value interface{}) bool {
		ws := value.(*webSocket)
		if err != nil {
			return true
		}
		ws.WriteMessage(websocket.TextMessage, data)
		return true
	})
}
func setListenerCount() {
	data, err := json.Marshal(map[string]interface{}{
		"op":        opSetClientsListeners,
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	connections.Range(func(key, value interface{}) bool {
		ws := value.(*webSocket)
		if err != nil {
			return true
		}
		ws.WriteMessage(websocket.TextMessage, data)
		return true
	})
}

//#endregion

//#region Data Request Handling

func getPlaying() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"op":        opSetClientsTrack,
		"track":     currentTrackMeta,
		"pos":       atomic.LoadInt64(&startPos),
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	return data
}

func getListenersCount() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"op":        opSetClientsListeners,
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	return data
}

//#endregion

//#region Queue

func enqueue(msg wsMessage) []byte {
	var err error
	if len(msg.Query) == 0 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": false,
			"reason":  "Invalid Query!",
		})
		return data
	}
	var tracks []common.Track
	log.Printf("Client Queried: %s", msg.Query)
	switch msg.Selector {
	case common.CSN:
		tracks, err = csn.Search(msg.Query)
	default:
		tracks, err = dzClient.SearchTrack(msg.Query, "")
	}
	switch {
	case err != nil:
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": false,
			"reason":  "Search Failed!",
		})
		return data
	case len(tracks) == 0:
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": false,
			"reason":  "No Result!",
		})
		return data
	default:
		track := tracks[0]
		spotifyURI := track.SpotifyURI()
		if track.Source() == common.Deezer {
			track, err = dzClient.GetTrackByID(track.ID())
			dtrack := track.(deezer.Track)
			dtrack.SetSpotifyURI(spotifyURI)
			track = dtrack
		}
		playQueue.Enqueue(track)
		enqueueCallback(track)
		log.Printf("Track enqueued: %v - %v\n", track.Title(), track.Artist())
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": true,
			"reason":  "",
			"track":   common.GetMetadata(track),
		})
		return data
	}

}

func skip() []byte {
	if atomic.LoadInt32(&isRadioStreaming) == 1 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestSkip,
			"success": false,
			"reason":  "You can't skip a radio stream.",
		})

		return data
	}
	if time.Since(startTime) < 5*time.Second {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestSkip,
			"success": false,
			"reason":  "Please wait until first 5 seconds has passed.",
		})
		return data
	}
	skipChannel <- 0
	log.Println("Current song skipped!")
	data, err := json.Marshal(map[string]interface{}{
		"op": opAllClientsSkip,
	})
	connections.Range(func(key, value interface{}) bool {
		ws := value.(*webSocket)
		if err != nil {
			return true
		}
		ws.WriteMessage(websocket.TextMessage, data)
		return true
	})
	data, _ = json.Marshal(map[string]interface{}{
		"op":      opClientRequestSkip,
		"success": true,
		"reason":  "",
	})
	return data
}
func enqueueCallback(value interface{}) {
	track := value.(common.Track)
	metadata := common.GetMetadata(track)
	cacheQueue.Enqueue(metadata)
	go func(metadata common.TrackMetadata) {
		log.Printf("Enqueuing track on all clients %v - %v\n", metadata.Title, metadata.Artist)
		data, err := json.Marshal(map[string]interface{}{
			"op":    opTrackEnqueued,
			"track": metadata,
		})
		connections.Range(func(key, value interface{}) bool {
			ws := value.(*webSocket)
			if err != nil {
				return false
			}
			ws.WriteMessage(websocket.TextMessage, data)
			return true
		})
	}(metadata)
}
func dequeueCallback() {
	cacheQueue.Dequeue()
}

func getQueue() []byte {
	elements := cacheQueue.GetElements()
	tracks := make([]common.TrackMetadata, len(elements))
	for i, val := range elements {
		tracks[i] = val.(common.TrackMetadata)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"op":    opClientRequestQueue,
		"queue": tracks,
	})

	return data
}
func removeTrack(msg wsMessage) []byte {
	removed := playQueue.Remove(func(value interface{}) bool {
		ele := value.(common.Track)
		if ele.PlayID() == msg.Query {
			return true
		}
		return false
	})
	var removedTrack common.TrackMetadata
	if removed != nil {
		removedTrack = cacheQueue.Remove(func(value interface{}) bool {
			ele := value.(common.TrackMetadata)
			if ele.PlayID == msg.Query {
				return true
			}
			return false
		}).(common.TrackMetadata)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"op":      opClientRemoveTrack,
		"success": removed != nil,
		"track":   removedTrack,
	})
	if removed != nil {
		connections.Range(func(key, value interface{}) bool {
			ws := value.(*webSocket)
			ws.WriteMessage(websocket.TextMessage, data)
			return true
		})
	}
	return data
}

//#endregion

//#region HTTP Handler

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

//#endregion

//#region Main Threads

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
	currentTrackID = -1
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
	go inactivityMonitor()
	<-initialized
	log.Printf("Serving on port %s", port)
	log.Fatal(http.ListenAndServe(port, logRequest(http.DefaultServeMux)))
}

//#endregion

//#region Utility Threads

func selfPinger() {
	appName, ok := os.LookupEnv("HEROKU_APP_NAME")
	if !ok {
		return
	}
	log.Println("Starting periodic keep-alive ping...")
	url := fmt.Sprintf("https://%s.herokuapp.com", appName)
	for {
		if atomic.LoadInt32(&listenersCount) > 0 {
			http.Get(url)
			log.Println("Ping!")
		}
		time.Sleep(1 * time.Minute)
	}
}

func listenerMonitor(ch chan int32) {
	timer := time.NewTimer(1 * time.Minute)
	for {
		if listeners := atomic.LoadInt32(&listenersCount); listeners > 0 {
			ch <- listeners
		}
		timer.Reset(1 * time.Minute)
		select {
		case <-newListenerC:
		case <-timer.C:
		}
	}
}

func inactivityMonitor() {
	timer := time.NewTimer(15 * time.Minute)
	lch := make(chan int32)
	go listenerMonitor(lch)
	isStandby := false
	for {
		select {
		case l := <-lch:
			timer.Reset(15 * time.Minute)
			if isStandby {
				if atomic.LoadInt32(&isRadioStreaming) > 0 {
					go processRadio(quitRadio)
				}
				activityWg.Done()
				isStandby = false
			}
			log.Println("Listeners: ", l)
		case <-timer.C:
			log.Println("Inactivity. Standby...")
			isStandby = true
			activityWg.Add(1)
			if atomic.LoadInt32(&isRadioStreaming) > 0 {
				quitRadio <- 0
			} else {
				skipChannel <- 1
			}
			deltaChannel <- 0
			setTrack(common.TrackMetadata{
				Title:   "Standby...",
				Artist:  "Inactivity",
				Artists: "The Stream is standby due to inactivity",
			})
			time.Sleep(5 * time.Second)
			<-quitRadio
		}
	}
}

//#endregion
