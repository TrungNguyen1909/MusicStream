package main

import (
	"bytes"
	"common"
	"deezer"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"lyrics"
	"net/http"
	"os"
	"queue"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"vorbisencoder"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/gorilla/websocket"
)

const (
	chunkDelayMS = 40
)

func mainTest() {
	fmt.Println("Hello World!")
	client := deezer.Client{}
	client.Init()
	track, err := client.DownloadTrack("637884472", 3)
	if err != nil {
		log.Fatal(err)
	}
	streamer, format, err := mp3.Decode(track)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()
	fmt.Println(format)
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	speaker.Play(streamer)
	select {}
}

type chunk struct {
	buffer      []byte
	encoderTime time.Duration
	channel     int
}
type webSocket struct {
	conn *websocket.Conn
	mux  *sync.Mutex
}

var gotNewBuffer *sync.Cond
var upgrader = websocket.Upgrader{}
var connections sync.Map
var currentTrack common.Track
var dzClient deezer.Client
var playQueue *queue.Queue
var channels [2]chan chan chunk
var currentChannel int
var oggHeader []byte
var listenersCount int32
var bufferingChannel chan chunk
var etaDone time.Time
var skipChannel chan int
var isRadioStreaming int32

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
	streamer, format, err := mp3.Decode(stream)
	if err != nil {
		log.Fatal(err)
	}
	defer streamer.Close()
	resampled := beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
	format.SampleRate = beep.SampleRate(48000)
	encoder := vorbisencoder.NewEncoder(2, 48000)
	encoder.Encode(oggHeader, make([]byte, 0))

	var encodedTime time.Duration
	for i := 0; i < 2; i++ {
		silenceFrame := make([]byte, 20000)
		n := encoder.Encode(silenceFrame, make([]byte, 76032))
		silenceFrame = silenceFrame[:n]
		encodedTime += 396 * time.Millisecond
		bufferingChannel <- chunk{buffer: silenceFrame, encoderTime: encodedTime}
	}
	defer encoder.Close()
	defer func() {
		for i := 0; i < 2; i++ {
			silenceFrame := make([]byte, 20000)
			n := encoder.Encode(silenceFrame, make([]byte, 76032))
			silenceFrame = silenceFrame[:n]
			encodedTime += 396 * time.Millisecond
			bufferingChannel <- chunk{buffer: silenceFrame, encoderTime: encodedTime}
		}
		bufferingChannel <- chunk{buffer: nil, encoderTime: 0}
	}()
	for {
		select {
		case <-quit:
			return
		default:
		}
		samples := make([][2]float64, 960)
		n, ok := resampled.Stream(samples)
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
	time.Sleep(time.Until(etaDone))
	log.Println("Radio preloading started!")
	req, err := http.NewRequest("GET", "https://listen.moe/stream", nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("authority", "listen.moe")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("dnt", "1")
	req.Header.Set("accept-encoding", "identity;q=1, *;q=0")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.117 Safari/537.36")
	req.Header.Set("accept", "*/*")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("referer", "https://listen.moe/")
	req.Header.Set("accept-language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("range", "bytes=0-")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { bufferingChannel <- chunk{buffer: nil, encoderTime: 0} }()
	defer log.Println("Radio preloading stopped!")
	encoder := vorbisencoder.NewEncoder(2, 48000)
	encoder.Encode(oggHeader, make([]byte, 0))
	defer encoder.Close()
start:
	streamer, format, err := vorbis.Decode(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	defer streamer.Close()
	var encodedTime time.Duration
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
	setTrack(common.Track{Title: "listen.moe"})
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
			log.Println("Panicked!!!:", r)
			log.Println("Creating a new deezer client...")
			dzClient = deezer.Client{}
			dzClient.Init()
			log.Println("Resuming...")
		}
	}()
	quitRadio := make(chan int, 10)
	radioStarted := false
	if playQueue.Empty() {
		radioStarted = true
		go processRadio(quitRadio)
	}
	track := playQueue.Pop().(common.Track)
	if radioStarted {
		quitRadio <- 0
	}
	log.Printf("Playing %v - %v\n", track.Title, track.Artist.Name)
	var mxmlyrics common.LyricsResult
	mxmlyrics, err := lyrics.GetLyrics(track.Title, track.Artist.Name, track.Album.Title, track.Album.Artist.Name, track.Duration)
	if err == nil {
		track.Lyrics = mxmlyrics
	}
	stream, err := dzClient.DownloadTrack(strconv.Itoa(track.ID), 3)
	if err != nil {
		log.Fatal(err)
	}
	quit := make(chan int, 10)
	go preloadTrack(stream, quit)
	if radioStarted {
		<-quitRadio
	}
	time.Sleep(time.Until(etaDone))
	setTrack(track)
	streamToClients(skipChannel, quit)
	log.Println("Stream ended!")
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
	encoder.Close()
	playQueue = queue.NewQueue()
	gotNewBuffer = sync.NewCond(&sync.Mutex{})
	dzClient = deezer.Client{}
	dzClient.Init()

	for {
		processTrack()
	}
}
func setTrack(track common.Track) {
	time.Sleep(1 * time.Second)
	currentTrack = track
	log.Printf("Setting track on all clients %v - %v\n", currentTrack.Title, currentTrack.Artist.Name)
	data, err := json.Marshal(map[string]interface{}{
		"op":    1,
		"track": track,
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

type wsMessage struct {
	Operation int    `json:"op"`
	Query     string `json:"query"`
}

func getPlaying() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"op":    1,
		"track": currentTrack,
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
	if len(msg.Query) == 0 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      3,
			"success": false,
			"reason":  "Invalid Query!",
		})
		return data
	}
	tracks, err := dzClient.SearchTrack(msg.Query, "")
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
		playQueue.Enqueue(track)
		log.Printf("Track enqueued: %v - %v\n", track.Title, track.Artist.Name)
		data, _ := json.Marshal(map[string]interface{}{
			"op":      3,
			"success": true,
			"reason":  "",
			"track":   track,
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
func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
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
	http.Handle("/", http.FileServer(http.Dir("static")))
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
