package common

import (
	"crypto/rand"
	"io"
	"net/http"
)

const (
	//Deezer Source
	Deezer = 1
	//CSN Source
	CSN = 2
	//Radio (listen.moe) Source
	Radio = 3
)

//Track represents a track from any sources
type Track interface {
	ID() int
	Source() int
	Title() string
	Artist() string
	Artists() string
	Album() string
	CoverURL() string
	Duration() int
	SpotifyURI() string
	PlayID() string
	Download() (io.ReadCloser, error)
}

//TrackMetadata contains essential informations about a track for client
type TrackMetadata struct {
	Title    string       `json:"title"`
	Source   int          `json:"source"`
	Duration int          `json:"duration"`
	Artist   string       `json:"artist"`
	Artists  string       `json:"artists"`
	Album    string       `json:"album"`
	CoverURL string       `json:"cover"`
	Lyrics   LyricsResult `json:"lyrics"`
	PlayID   string       `json:"playId"`
}

//GetMetadata returns a new TrackMetadata created from a provided Track
func GetMetadata(track Track) (d TrackMetadata) {
	d = TrackMetadata{}
	d.Title = track.Title()
	d.Source = track.Source()
	d.Duration = track.Duration()
	d.Artist = track.Artist()
	d.Artists = track.Artists()
	d.Album = track.Album()
	d.CoverURL = track.CoverURL()
	d.PlayID = track.PlayID()
	return
}

//LyricsTime represents the time that the lyrics will be shown
type LyricsTime struct {
	Hundredths int     `json:"hundredths"`
	Minutes    int     `json:"minutes"`
	Seconds    int     `json:"seconds"`
	Total      float64 `json:"total"`
}

//LyricsLine contains informations about a piece of lyrics
type LyricsLine struct {
	Text       string     `json:"text"`
	Translated string     `json:"translated"`
	Time       LyricsTime `json:"time"`
	Original   string     `json:"original"`
}

//LyricsResult represents a result of a lyrics query
type LyricsResult struct {
	RawLyrics    string       `json:"txt"`
	SyncedLyrics []LyricsLine `json:"lrc"`
	Language     string       `json:"lang"`
}

//RadioTrack is a special track from radio sources
type RadioTrack struct {
	title  string
	artist string
	album  string
}

//SetTitle sets the title of currently playing track on radio, if known, otherwise, leave the radio's name
func (track *RadioTrack) SetTitle(title string) {
	track.title = title
}

//SetArtist sets the artist of currently playing track on radio
func (track *RadioTrack) SetArtist(artist string) {
	track.artist = artist
}

//SetAlbum sets the album of currently playing track on radio
func (track *RadioTrack) SetAlbum(album string) {
	track.album = album
}

//ID returns the ID number of currently playing track on radio, if known, otherwise, returns 0
func (track RadioTrack) ID() int {
	return 0
}

//Title returns the title of currently playing track on radio, if known, otherwise, returns the radio's name
func (track RadioTrack) Title() string {
	return track.title
}

//Album returns the album's name of currently playing track on radio, if known
func (track RadioTrack) Album() string {
	return track.album
}

//Source returns Radio
func (track RadioTrack) Source() int {
	return Radio
}

//Artist returns the artist's name of currently playing track on radio, if known
func (track RadioTrack) Artist() string {
	return track.artist
}

//Artists returns the artist's name of currently playing track on radio, if known
func (track RadioTrack) Artists() string {
	return track.artist
}

//Duration returns the duration of currently playing track on radio, if known, otherwise, 0
func (track RadioTrack) Duration() int {
	return 0
}

//CoverURL returns the URL to the cover of currently playing track on radio, if known
func (track RadioTrack) CoverURL() string {
	return ""
}

//SpotifyURI returns the currently playing track's equivalent spotify song, if known
func (track RadioTrack) SpotifyURI() string {
	return ""
}

//Download returns an mp3 stream to the radio
func (track RadioTrack) Download() (stream io.ReadCloser, err error) {
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
func (track RadioTrack) PlayID() string {
	return ""
}

const alphabet string = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"

//GenerateID generates an unique alphanumberic string
func GenerateID() (id string) {
	b := make([]byte, 8)
	rand.Read(b)
	for _, v := range b {
		id += string(alphabet[int(v)%len(alphabet)])
	}
	return
}
