package main

import (
	"bytes"
	"common"
	"crypto/sha1"
	"csn"
	"deezer"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"lyrics"
	"net/http"
	"os"
	"path"
	"queue"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"vorbisencoder"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
	"github.com/gorilla/websocket"
)

const (
	chunkDelayMS = 40
)

type chunk struct {
	buffer      []byte
	encoderTime time.Duration
	channel     int
}
type webSocket struct {
	conn *websocket.Conn
	mux  *sync.Mutex
}
type wsMessage struct {
	Operation int    `json:"op"`
	Query     string `json:"query"`
	Selector  int    `json:"selector"`
}

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
var etaDone time.Time
var skipChannel chan int
var isRadioStreaming int32
var currentTrackID int
var watchDog int
var radio common.RadioTrack

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
func streamToClients(quit chan int, quitPreload chan int) {

	start := time.Now()
	etaDone = start
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
			etaDone = start.Add(Chunk.encoderTime)
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
func preloadTrack(stream io.ReadCloser, quit chan int) {
	var encodedTime time.Duration
	streamer, format, err := mp3.Decode(stream)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()
	var needResampling bool
	var resampled *beep.Resampler
	if format.SampleRate != beep.SampleRate(48000) {
		resampled = beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
		format.SampleRate = beep.SampleRate(48000)
		needResampling = true
	}
	encoder := vorbisencoder.NewEncoder(2, 48000)
	encoder.Encode(oggHeader, make([]byte, 0))
	bufferingChannel <- chunk{buffer: oggHeader, encoderTime: encodedTime}
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			silenceFrame := make([]byte, 20000)
			n := encoder.Encode(silenceFrame, make([]byte, 76032))
			silenceFrame = silenceFrame[:n]
			encodedTime += 396 * time.Millisecond
			bufferingChannel <- chunk{buffer: silenceFrame, encoderTime: encodedTime}
		}
	}
	defer func() {
		for j := 0; j < 2; j++ {
			for i := 0; i < 2; i++ {
				silenceFrame := make([]byte, 20000)
				n := encoder.Encode(silenceFrame, make([]byte, 76032))
				silenceFrame = silenceFrame[:n]
				encodedTime += 396 * time.Millisecond
				bufferingChannel <- chunk{buffer: silenceFrame, encoderTime: encodedTime}
			}
		}
		lastBuffer := make([]byte, 20000)
		n := encoder.EndStream(lastBuffer)
		bufferingChannel <- chunk{buffer: lastBuffer[:n], encoderTime: encodedTime}
		bufferingChannel <- chunk{buffer: nil, encoderTime: 0}
	}()
	for {
		select {
		case <-quit:
			return
		default:
		}
		samples := make([][2]float64, 960)
		var n int
		var ok bool
		if needResampling {
			n, ok = resampled.Stream(samples)
		} else {
			n, ok = streamer.Stream(samples)
		}
		if !ok {
			break
		}
		var buf bytes.Buffer
		for _, sample := range samples {
			p := make([]byte, format.Width())
			format.EncodeSigned(p, sample)
			for _, v := range p {
				buf.WriteByte(v)
			}
		}
		output := make([]byte, 10000)
		encodedLen := encoder.Encode(output, buf.Bytes())
		encodedTime += 20 * time.Millisecond
		if encodedLen > 0 {
			bufferingChannel <- chunk{buffer: output[:encodedLen], encoderTime: encodedTime}
		}
		if 0 <= n && n < 960 && ok {
			break
		}
	}
}
func preloadRadio(quit chan int) {
	var encodedTime time.Duration
	time.Sleep(time.Until(etaDone))
	log.Println("Radio preloading started!")
	stream, _ := radio.Download()
	defer stream.Close()
	defer func() {
		bufferingChannel <- chunk{buffer: nil, encoderTime: 0}
	}()
	defer log.Println("Radio preloading stopped!")
	encoder := vorbisencoder.NewEncoder(2, 48000)
	encoder.Encode(oggHeader, make([]byte, 0))
	bufferingChannel <- chunk{buffer: oggHeader, encoderTime: encodedTime}
	defer func() {

		lastBuffer := make([]byte, 10000)
		n := encoder.EndStream(lastBuffer)
		bufferingChannel <- chunk{buffer: lastBuffer[:n], encoderTime: encodedTime}
	}()
start:
	streamer, format, err := vorbis.Decode(stream)
	if err != nil {
		log.Fatal(err)
	}

	defer streamer.Close()
	for {
		select {
		case <-quit:
			return
		default:
		}
		samples := make([][2]float64, 960)
		n, ok := streamer.Stream(samples)
		if !ok {
			goto start
		}
		var buf bytes.Buffer
		for _, sample := range samples {
			p := make([]byte, format.Width())
			format.EncodeSigned(p, sample)
			for _, v := range p {
				buf.WriteByte(v)
			}
		}
		output := make([]byte, 10000)
		encodedLen := encoder.Encode(output, buf.Bytes())
		encodedTime += 20 * time.Millisecond
		if encodedLen > 0 {
			bufferingChannel <- chunk{buffer: output[:encodedLen], encoderTime: encodedTime}
		}
		if 0 <= n && n < 960 && ok {
			goto start
		}
	}
}
func processRadio(quit chan int) {
	quitPreload := make(chan int, 10)
	time.Sleep(time.Until(etaDone))
	currentTrack = radio
	setTrack(common.GetMetadata(radio))
	go preloadRadio(quitPreload)
	atomic.StoreInt32(&isRadioStreaming, 1)
	defer atomic.StoreInt32(&isRadioStreaming, 0)
	defer log.Println("Radio stream ended")
	defer func() { log.Println("Resuming track streaming..."); quit <- 0 }()
	streamToClients(quit, quitPreload)
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
	quitRadio := make(chan int, 10)
	if currentTrackID == -1 || watchDog >= 3 || currentTrack.Source() == common.CSN {
		if playQueue.Empty() {
			radioStarted = true
			go processRadio(quitRadio)
		}
		track = playQueue.Pop().(common.Track)
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
	currentTrackID = track.ID()
	currentTrack = track
	if radioStarted {
		quitRadio <- 0
	}
	log.Printf("Playing %v - %v\n", track.Title(), track.Artist())
	trackDict := common.GetMetadata(track)
	var mxmlyrics common.LyricsResult
	mxmlyrics, err = lyrics.GetLyrics(track.Title(), track.Artist(), track.Album(), track.Artists(), track.Duration())
	if err == nil {
		trackDict.Lyrics = mxmlyrics
	}
	stream, err := track.Download()
	if err != nil {
		panic(err)
	}
	quit := make(chan int, 10)
	go preloadTrack(stream, quit)
	if radioStarted {
		<-quitRadio
	}
	for len(skipChannel) > 0 {
		select {
		case <-skipChannel:
		default:
		}
	}
	time.Sleep(time.Until(etaDone))
	setTrack(trackDict)
	streamToClients(skipChannel, quit)
	log.Println("Stream ended!")
	currentTrackID = -1
	watchDog = 0
}

func audioManager() {
	for i := range channels {
		channels[i] = make(chan chan chunk, 1000)
	}
	bufferingChannel = make(chan chunk, 500)
	skipChannel = make(chan int, 500)
	encoder := vorbisencoder.NewEncoder(2, 48000)
	oggHeader = make([]byte, 5000)
	n := encoder.Encode(oggHeader, make([]byte, 0))
	oggHeader = oggHeader[:n]
	encoder.EndStream(nil)
	playQueue = queue.NewQueue()
	dzClient = deezer.NewClient()
	radio = common.RadioTrack{}
	currentTrackID = -1
	for {
		processTrack()
	}
}
func setTrack(trackMeta common.TrackMetadata) {
	time.Sleep(1 * time.Second)
	currentTrackMeta = trackMeta
	log.Printf("Setting track on all clients %v - %v\n", trackMeta.Title, trackMeta.Artist)
	data, err := json.Marshal(map[string]interface{}{
		"op":    1,
		"track": trackMeta,
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
		"op":        5,
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

func audioHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Fatal("expected http.ResponseWriter to be an http.Flusher")
	}
	atomic.AddInt32(&listenersCount, 1)
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

func getPlaying() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"op":    1,
		"track": currentTrackMeta,
	})
	return data
}

func getListenersCount() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"op":        5,
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	return data
}
func enqueue(msg wsMessage) []byte {
	var err error
	if len(msg.Query) == 0 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      3,
			"success": false,
			"reason":  "Invalid Query!",
		})
		return data
	}
	var tracks []common.Track
	switch msg.Selector {
	case 2:
		tracks, err = csn.Search(msg.Query)
	default:
		tracks, err = dzClient.SearchTrack(msg.Query, "")
	}
	switch {
	case err != nil:
		data, _ := json.Marshal(map[string]interface{}{
			"op":      3,
			"success": false,
			"reason":  "Search Failed!",
		})
		return data
	case len(tracks) == 0:
		data, _ := json.Marshal(map[string]interface{}{
			"op":      3,
			"success": false,
			"reason":  "No Result!",
		})
		return data
	default:
		track := tracks[0]
		if track.Source() == common.Deezer {
			track, err = dzClient.GetTrackByID(track.ID())
		} else if track.Source() == common.CSN {
			cTrack := track.(csn.Track)
			err = cTrack.Populate()
			track = cTrack
		}
		playQueue.Enqueue(track)
		log.Printf("Track enqueued: %v - %v\n", track.Title(), track.Artist())
		data, _ := json.Marshal(map[string]interface{}{
			"op":      3,
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
			"op":      4,
			"success": false,
			"reason":  "You can't skip a radio stream.",
		})

		return data
	}
	skipChannel <- 0
	log.Println("Current song skipped!")
	data, _ := json.Marshal(map[string]interface{}{
		"op":      4,
		"success": true,
		"reason":  "",
	})

	return data
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
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
		case 1:
			c.WriteMessage(websocket.TextMessage, getPlaying())
		case 3:
			c.WriteMessage(websocket.TextMessage, enqueue(msg))
		case 4:
			c.WriteMessage(websocket.TextMessage, skip())
		case 5:
			c.WriteMessage(websocket.TextMessage, getListenersCount())
		case 8:
			data, _ := json.Marshal(map[string]interface{}{
				"op": 8,
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
			"op":      3,
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
		etag := sha1.Sum(content)
		w.Header().Set("ETag", "W/"+fmt.Sprintf("%x", etag))
		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, w.Header().Get("ETag")) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		http.ServeContent(w, r, d.Name(), d.ModTime(), f)
		return

	}
}
func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		w.Header().Set("Cache-Control", "no-cache")
		handler.ServeHTTP(w, r)
	})
}
func main() {
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8890"
	}
	port = ":" + port
	http.HandleFunc("/enqueue", enqueueHandler)
	http.HandleFunc("/listeners", listenersHandler)
	http.HandleFunc("/audio", audioHandler)
	http.HandleFunc("/status", wsHandler)
	http.HandleFunc("/playing", playingHandler)
	http.HandleFunc("/skip", skipHandler)
	http.HandleFunc("/", fileServer(http.Dir("www")))
	go audioManager()
	go selfPinger()
	log.Fatal(http.ListenAndServe(port, logRequest(http.DefaultServeMux)))
}

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
