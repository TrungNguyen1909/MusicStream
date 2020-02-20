package radio

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/gorilla/websocket"
)

//Track is a special track from radio sources
type Track struct {
	id                 int
	title              string
	artist             string
	artists            string
	album              string
	ws                 *websocket.Conn
	heartbeatInterval  int
	heartbeatInterrupt chan int
	trackUpdateEvent   *sync.Cond
	mux                sync.RWMutex
}

//setArtists sets the contributors of currently playing track on radio
func (track *Track) setArtists(artists []string) {
	track.artists = strings.Join(artists, ", ")
}

//ID returns the ID number of currently playing track on radio, if known, otherwise, returns 0
func (track *Track) ID() int {
	track.mux.RLock()
	defer track.mux.RUnlock()
	return track.id
}

//Title returns the title of currently playing track on radio, if known, otherwise, returns the radio's name
func (track *Track) Title() string {
	track.mux.RLock()
	defer track.mux.RUnlock()
	return track.title
}

//Album returns the album's name of currently playing track on radio, if known
func (track *Track) Album() string {
	track.mux.RLock()
	defer track.mux.RUnlock()
	return track.album
}

//Source returns Radio
func (track *Track) Source() int {
	return common.Radio
}

//Artist returns the artist's name of currently playing track on radio, if known
func (track *Track) Artist() string {
	track.mux.RLock()
	defer track.mux.RUnlock()
	return track.artist
}

//Artists returns the artist's name of currently playing track on radio, if known
func (track *Track) Artists() string {
	track.mux.RLock()
	defer track.mux.RUnlock()
	return track.artists
}

//Duration returns the duration of currently playing track on radio, if known, otherwise, 0
func (track *Track) Duration() int {
	return 0
}

//CoverURL returns the URL to the cover of currently playing track on radio, if known
func (track *Track) CoverURL() string {
	return ""
}

//SpotifyURI returns the currently playing track's equivalent spotify song, if known
func (track *Track) SpotifyURI() string {
	return ""
}

//Download returns an mp3 stream to the radio
func (track *Track) Download() (stream io.ReadCloser, err error) {
	req, err := http.NewRequest("GET", "https://listen.moe/stream", nil)
	if err != nil {
		return
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
		return
	}
	stream = resp.Body
	return
}

//PlayID returns 0
func (track *Track) PlayID() string {
	return ""
}
func (track *Track) heartbeat() {
	for len(track.heartbeatInterrupt) > 0 {
		<-track.heartbeatInterrupt
	}
	ticker := time.NewTicker((time.Duration)(track.heartbeatInterval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := track.ws.WriteJSON(map[string]interface{}{"op": 9})
			if err != nil {
				log.Printf("Track:heartbeat: %#v", err)
				return
			}
		case <-track.heartbeatInterrupt:
			return
		}
	}
}

//WaitForTrackUpdate waits until a new track update event from WS broadcast
func (track *Track) WaitForTrackUpdate() {
	track.trackUpdateEvent.L.Lock()
	defer track.trackUpdateEvent.L.Unlock()
	track.trackUpdateEvent.Wait()
}
func (track *Track) initWS() {
	track.heartbeatInterrupt = make(chan int, 1)
	u := url.URL{Scheme: "wss", Host: "listen.moe", Path: "/gateway_v2"}
	log.Printf("connecting to %s", u.String())

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	track.ws = ws
	go func() {
		defer ws.Close()
		for {
			var msg message
			err := ws.ReadJSON(&msg)
			if err != nil {
				log.Println("Track:readJSON:", err)
				return
			}
			if msg.Data == nil {
				continue
			}
			switch msg.Operation {
			case 0:
				data := msg.Data.(*heartbeatData)
				track.heartbeatInterval = data.Heartbeat
				track.heartbeatInterrupt <- 1
				go track.heartbeat()
			case 1:
				if msg.EventType != "TRACK_UPDATE" && msg.EventType != "TRACK_UPDATE_REQUEST" {
					continue
				}
				data := msg.Data.(*playbackData)
				track.mux.Lock()
				track.id = data.Song.ID
				track.title = data.Song.Title
				if len(data.Song.Albums) > 0 {
					track.album = data.Song.Albums[0].Name
				}
				if len(data.Song.Artists) > 0 {
					track.artist = data.Song.Artists[0].Name
					artists := make([]string, 0, len(data.Song.Artists))
					for _, artist := range data.Song.Artists {
						artists = append(artists, artist.Name)
					}
					track.setArtists(artists)
				}
				track.mux.Unlock()
				track.trackUpdateEvent.Broadcast()
			}
		}
	}()
}

//NewTrack returns an initialized Radio.Track
func NewTrack() (radio *Track) {
	radio = &Track{title: "listen.moe", trackUpdateEvent: sync.NewCond(&sync.Mutex{})}
	radio.initWS()
	return
}

type message struct {
	Operation int         `json:"op"`
	Data      interface{} `json:"d"`
	EventType string      `json:"t"`
}
type heartbeatData struct {
	Message   string `json:"message"`
	Heartbeat int    `json:"heartbeat"`
}
type playbackData struct {
	Event      interface{} `json:"event"`
	LastPlayed []struct {
		Albums []struct {
			ID         int         `json:"id"`
			Image      interface{} `json:"image"`
			Name       string      `json:"name"`
			NameRomaji interface{} `json:"nameRomaji"`
		} `json:"albums"`
		Artists []struct {
			ID         int         `json:"id"`
			Image      interface{} `json:"image"`
			Name       string      `json:"name"`
			NameRomaji interface{} `json:"nameRomaji"`
		} `json:"artists"`
		Duration int           `json:"duration"`
		Favorite bool          `json:"favorite"`
		ID       int           `json:"id"`
		Sources  []interface{} `json:"sources"`
		Title    string        `json:"title"`
	} `json:"lastPlayed"`
	Listeners int         `json:"listeners"`
	Requester interface{} `json:"requester"`
	Song      struct {
		Albums []struct {
			ID         int         `json:"id"`
			Image      interface{} `json:"image"`
			Name       string      `json:"name"`
			NameRomaji interface{} `json:"nameRomaji"`
		} `json:"albums"`
		Artists []struct {
			ID         int         `json:"id"`
			Image      string      `json:"image"`
			Name       string      `json:"name"`
			NameRomaji interface{} `json:"nameRomaji"`
		} `json:"artists"`
		Duration int           `json:"duration"`
		Favorite bool          `json:"favorite"`
		ID       int           `json:"id"`
		Sources  []interface{} `json:"sources"`
		Title    string        `json:"title"`
	} `json:"song"`
	StartTime string `json:"startTime"`
}

func (d *message) UnmarshalJSON(data []byte) error {
	var msg struct {
		Operation int              `json:"op"`
		Data      *json.RawMessage `json:"d"`
		EventType string           `json:"t"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	switch msg.Operation {
	case 0:
		d.Data = new(heartbeatData)
	case 1:
		d.Data = new(playbackData)
	default:
		return nil
	}
	d.Operation = msg.Operation
	d.EventType = msg.EventType
	return json.Unmarshal(*msg.Data, &d.Data)
}
