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

	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gorilla/websocket"
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
}
type WebSocket struct {
	conn *websocket.Conn
	mux  *sync.Mutex
}

var currentBuffer []byte
var gotNewBuffer *sync.Cond
var upgrader = websocket.Upgrader{}
var connections sync.Map
var currentTrack deezer.Track
var dzClient deezer.Client
var playQueue *queue.Queue
var channels [2]chan chan int
var currentChannel int
var oggHeader []byte
var listenersCount int32
var bufferingChannel chan chunk

func (socket *WebSocket) WriteMessage(messageType int, data []byte) error {
	socket.mux.Lock()
	defer socket.mux.Unlock()
	return socket.conn.WriteMessage(messageType, data)
}
func (socket *WebSocket) Close() error {
	socket.mux.Lock()
	defer socket.mux.Unlock()
	return socket.conn.Close()
}
func (socket *WebSocket) ReadJSON(v interface{}) error {
	return socket.conn.ReadJSON(v)
}
func preloadTrack(stream io.ReadCloser) {
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
			currentBuffer = output[:encodedLen]
			bufferingChannel <- chunk{buffer: currentBuffer, encoderTime: encodedTime}
		}
		// err = binary.Write(w, binary.BigEndian, buf.Bytes())
		if 0 <= n && n < 882 && ok {
			break
		}
	}
}
func processTrack() {
	if playQueue.Empty() {
		setTrack(deezer.Track{})
	}
	track := playQueue.Pop().(deezer.Track)
	setTrack(track)
	fmt.Println(track.Title)
	fmt.Println(track.Artist.Name)
	fmt.Println(track.Album.Title)
	stream, err := dzClient.DownloadTrack(strconv.Itoa(track.ID), 3)
	if err != nil {
		log.Fatal(err)
	}
	go preloadTrack(stream)
	start := time.Now()
	for {
		Chunk := <-bufferingChannel
		if Chunk.buffer == nil {
			break
		}
		currentBuffer = Chunk.buffer
		done := false
		for !done {
			select {
			case c := <-channels[currentChannel]:
				select {
				case c <- ((currentChannel + 1) % 2):
				default:
				}
			default:
				currentChannel = (currentChannel + 1) % 2
				done = true
			}
		}
		time.Sleep(Chunk.encoderTime - time.Since(start) - 40*time.Millisecond)
	}
	//time.Sleep((time.Duration)(track.Duration)*time.Second - time.Since(start))
	log.Println("Stream ended!")
}

func audioManager() {
	for i := range channels {
		channels[i] = make(chan chan int, 1000)
	}
	bufferingChannel = make(chan chunk, 1000)
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
	for {
		processTrack()
	}
}
func setTrack(track deezer.Track) {
	currentTrack = track
	data, err := json.Marshal(map[string]interface{}{
		"op":    1,
		"track": track,
	})
	connections.Range(func(key, value interface{}) bool {
		ws := value.(*WebSocket)
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
		ws := value.(*WebSocket)
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
	channel := make(chan int, 500)
	channels[currentChannel] <- channel
	chanidx := currentChannel
	for {
		chanidx = <-channel
		_, err := w.Write(currentBuffer)
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
	c := &WebSocket{conn: _c, mux: &sync.Mutex{}}
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

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]
	data, err := ioutil.ReadFile(strings.Join([]string{"static/new/", string(path)}, ""))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, public, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	if err != nil {
		http.ServeFile(w, r, "static/new/index.html")
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
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/new"))))
	http.HandleFunc("/", handler)
	go audioManager()
	log.Fatal(http.ListenAndServe(port, nil))
}
