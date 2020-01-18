package main

import (
	"bytes"
	"deezer"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"queue"
	"strconv"
	"strings"
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

//var currentBuffer []byte
var gotNewBuffer *sync.Cond
var upgrader = websocket.Upgrader{}
var connections sync.Map
var currentTrack deezer.Track
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
			//currentBuffer = Chunk.buffer
			if Chunk.buffer == nil {
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
	// resampled := beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
	// format.SampleRate = beep.SampleRate(48000)
	defer streamer.Close()

	encoder := vorbisencoder.NewEncoder(2, 44100)
	encoder.Encode(oggHeader, make([]byte, 0))
	defer encoder.Close()
	var encodedTime time.Duration
	defer func() { bufferingChannel <- chunk{buffer: nil, encoderTime: 0} }()
	for {
		select {
		case <-quit:
			return
		default:
		}
		samples := make([][2]float64, 882)
		n, ok := streamer.Stream(samples)
		if !ok {
			break
		}
		//data := make([]byte, format.Width()*n)
		var buf bytes.Buffer
		for _, sample := range samples {
			p := make([]byte, format.Width())
			format.EncodeSigned(p, sample)
			for _, v := range p {
				buf.WriteByte(v)
			}
		}
		output := make([]byte, 5000)
		encodedLen := encoder.Encode(output, buf.Bytes())
		//err = binary.Write(w, binary.BigEndian, output[:encodedLen])
		//log.Println(encodedLen)
		encodedTime += 20 * time.Millisecond
		if encodedLen > 0 {
			bufferingChannel <- chunk{buffer: output[:encodedLen], encoderTime: encodedTime}
		}
		// err = binary.Write(w, binary.BigEndian, buf.Bytes())
		if 0 <= n && n < 882 && ok {
			break
		}
	}
}
func preloadRadio(quit chan int) {
	time.Sleep(1 * time.Second)
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
start:
	streamer, format, err := vorbis.Decode(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	resampled := beep.Resample(4, format.SampleRate, beep.SampleRate(44100), streamer)
	format.SampleRate = beep.SampleRate(48000)
	defer streamer.Close()
	encoder := vorbisencoder.NewEncoder(2, 44100)
	encoder.Encode(oggHeader, make([]byte, 0))
	defer encoder.Close()
	var encodedTime time.Duration
	for {
		select {
		case <-quit:
			return
		default:
		}
		samples := make([][2]float64, 882)
		n, ok := resampled.Stream(samples)
		if !ok {
			goto start
		}
		//data := make([]byte, format.Width()*n)
		var buf bytes.Buffer
		for _, sample := range samples {
			p := make([]byte, format.Width())
			format.EncodeSigned(p, sample)
			for _, v := range p {
				buf.WriteByte(v)
			}
		}
		output := make([]byte, 5000)
		encodedLen := encoder.Encode(output, buf.Bytes())
		//err = binary.Write(w, binary.BigEndian, output[:encodedLen])
		//log.Println(encodedLen)
		encodedTime += 20 * time.Millisecond
		if encodedLen > 0 {
			bufferingChannel <- chunk{buffer: output[:encodedLen], encoderTime: encodedTime}
		}
		// err = binary.Write(w, binary.BigEndian, buf.Bytes())
		if 0 <= n && n < 882 && ok {
			goto start
		}
	}
}
func processRadio(quit chan int) {
	quitPreload := make(chan int, 10)
	time.Sleep(time.Until(etaDone))
	go preloadRadio(quitPreload)
	atomic.StoreInt32(&isRadioStreaming, 1)
	defer atomic.StoreInt32(&isRadioStreaming, 0)
	defer log.Println("Radio stream ended")
	defer func() { log.Println("Resuming track streaming..."); quit <- 0 }()
	streamToClients(quit, quitPreload)
}
func processTrack() {
	quitRadio := make(chan int, 10)
	radioStarted := false
	if playQueue.Empty() {
		setTrack(deezer.Track{Title: "listen.moe"})
		radioStarted = true
		go processRadio(quitRadio)
	}
	track := playQueue.Pop().(deezer.Track)
	if radioStarted {
		quitRadio <- 0
	}
	fmt.Println(track.Title)
	fmt.Println(track.Artist.Name)
	fmt.Println(track.Album.Title)
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
	//time.Sleep((time.Duration)(track.Duration)*time.Second - time.Since(start))
	log.Println("Stream ended!")
}

func audioManager() {
	for i := range channels {
		channels[i] = make(chan chan chunk, 1000)
	}
	bufferingChannel = make(chan chunk, 500)
	skipChannel = make(chan int, 500)
	encoder := vorbisencoder.NewEncoder(2, 44100)
	oggHeader = make([]byte, 5000)
	n := encoder.Encode(oggHeader, make([]byte, 0))
	oggHeader = oggHeader[:n]
	encoder.Close()
	playQueue = queue.NewQueue()
	gotNewBuffer = sync.NewCond(&sync.Mutex{})
	dzClient = deezer.Client{}
	dzClient.Init()
	//tracks, _ := dzClient.SearchTrack("Scared to be lonely", "")
	//playQueue.Enqueue(tracks[0])

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in audioManager:", r)
		}
	}()
	for {
		processTrack()
	}
}
func setTrack(track deezer.Track) {
	currentTrack = track
	log.Println("Setting track on all clients ", currentTrack)
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
	fmt.Printf("/audio %v\n", r.RemoteAddr)
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
			data, _ := json.Marshal(map[string]interface{}{
				"op":    1,
				"track": currentTrack,
			})
			c.WriteMessage(websocket.TextMessage, data)
		case 3:
			if len(msg.Query) == 0 {
				data, _ := json.Marshal(map[string]interface{}{
					"op":      3,
					"success": false,
					"reason":  "Invalid Query!",
				})
				c.WriteMessage(websocket.TextMessage, data)
			} else {
				tracks, err := dzClient.SearchTrack(msg.Query, "")
				switch {
				case err != nil:
					data, _ := json.Marshal(map[string]interface{}{
						"op":      3,
						"success": false,
						"reason":  "Search Failed!",
					})
					c.WriteMessage(websocket.TextMessage, data)
				case len(tracks) == 0:
					data, _ := json.Marshal(map[string]interface{}{
						"op":      3,
						"success": false,
						"reason":  "No Result!",
					})
					c.WriteMessage(websocket.TextMessage, data)
				default:
					track := tracks[0]
					playQueue.Enqueue(track)
					data, _ := json.Marshal(map[string]interface{}{
						"op":      3,
						"success": true,
						"reason":  "",
						"track":   track,
					})
					c.WriteMessage(websocket.TextMessage, data)
				}
			}
		case 4:
			if atomic.LoadInt32(&isRadioStreaming) == 1 {
				data, _ := json.Marshal(map[string]interface{}{
					"op":      4,
					"success": false,
					"reason":  "You can't skip a radio stream.",
				})
				c.WriteMessage(websocket.TextMessage, data)
			} else {
				skipChannel <- 0
				data, _ := json.Marshal(map[string]interface{}{
					"op":      4,
					"success": true,
					"reason":  "",
				})
				c.WriteMessage(websocket.TextMessage, data)
			}
		case 5:
			data, _ := json.Marshal(map[string]interface{}{
				"op":        5,
				"listeners": atomic.LoadInt32(&listenersCount),
			})
			c.WriteMessage(websocket.TextMessage, data)
		case 8:
			data, _ := json.Marshal(map[string]interface{}{
				"op": 8,
			})
			c.WriteMessage(websocket.TextMessage, data)

		}
	}

}

func playingHandler(w http.ResponseWriter, r *http.Request) {
	data, _ := json.Marshal(map[string]interface{}{
		"op":    1,
		"track": currentTrack,
	})
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(data)
	return
}

func listenersHandler(w http.ResponseWriter, r *http.Request) {
	data, _ := json.Marshal(map[string]interface{}{
		"op":        5,
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(data)
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
	if len(msg.Query) == 0 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      3,
			"success": false,
			"reason":  "Invalid Query!",
		})
		w.WriteHeader(http.StatusBadRequest)
		w.Write(data)
	} else {
		tracks, err := dzClient.SearchTrack(msg.Query, "")
		switch {
		case err != nil:
			data, _ := json.Marshal(map[string]interface{}{
				"op":      3,
				"success": false,
				"reason":  "Search Failed!",
			})
			w.Write(data)
		case len(tracks) == 0:
			data, _ := json.Marshal(map[string]interface{}{
				"op":      3,
				"success": false,
				"reason":  "No Result!",
			})
			w.Write(data)
		default:
			track := tracks[0]
			playQueue.Enqueue(track)
			data, _ := json.Marshal(map[string]interface{}{
				"op":      3,
				"success": true,
				"reason":  "",
				"track":   track,
			})
			w.Write(data)
		}
	}
}
func skipHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	if atomic.LoadInt32(&isRadioStreaming) == 1 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      4,
			"success": false,
			"reason":  "You can't skip a radio stream.",
		})

		w.Write(data)
	} else {
		skipChannel <- 0
		data, _ := json.Marshal(map[string]interface{}{
			"op":      4,
			"success": true,
			"reason":  "",
		})

		w.Write(data)
	}
}
func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]
	data, err := ioutil.ReadFile(strings.Join([]string{"static/", string(path)}, ""))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	if err != nil {
		http.ServeFile(w, r, "static/index.html")
		return
	}
	w.Write(data)
	return
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
	log.Fatal(http.ListenAndServe(port, nil))
}
