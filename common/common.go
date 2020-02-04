package common

import (
	"io"
	"net/http"
)

const (
	Deezer = 1
	CSN    = 2
	Radio  = 3
)

type Track interface {
	ID() int
	Source() int
	Title() string
	Artist() string
	Artists() string
	Album() string
	CoverURL() string
	Duration() int
	Download() (io.ReadCloser, error)
}

type TrackMetadata struct {
	Title    string       `json:"title"`
	Source   int          `json:"source"`
	Duration int          `json:"duration"`
	Artist   string       `json:"artist"`
	Artists  string       `json:"artists"`
	Album    string       `json:"album"`
	CoverURL string       `json:"cover"`
	Lyrics   LyricsResult `json:"lyrics"`
}

func GetMetadata(track Track) (d TrackMetadata) {
	d = TrackMetadata{}
	d.Title = track.Title()
	d.Source = track.Source()
	d.Duration = track.Duration()
	d.Artist = track.Artist()
	d.Artists = track.Artists()
	d.Album = track.Album()
	d.CoverURL = track.CoverURL()
	return
}

type LyricsTime struct {
	Hundredths int     `json:"hundredths"`
	Minutes    int     `json:"minutes"`
	Seconds    int     `json:"seconds"`
	Total      float64 `json:"total"`
}
type LyricsLine struct {
	Text       string     `json:"text"`
	Translated string     `json:"translated"`
	Time       LyricsTime `json:"time"`
	Original   string     `json:"original"`
}

type LyricsResult struct {
	RawLyrics    string       `json:"txt"`
	SyncedLyrics []LyricsLine `json:"lrc"`
	Language     string       `json:"lang"`
}

type RadioTrack struct {
	title  string
	artist string
	album  string
}

func (track *RadioTrack) SetTitle(title string) {
	track.title = title
}

func (track *RadioTrack) SetArtist(artist string) {
	track.artist = artist
}

func (track *RadioTrack) SetAlbum(album string) {
	track.album = album
}

func (track RadioTrack) ID() int {
	return 0
}

func (track RadioTrack) Title() string {
	return track.title
}

func (track RadioTrack) Album() string {
	return track.album
}

func (track RadioTrack) Source() int {
	return Radio
}

func (track RadioTrack) Artist() string {
	return track.artist
}
func (track RadioTrack) Artists() string {
	return track.artist
}
func (track RadioTrack) Duration() int {
	return 0
}

func (track RadioTrack) CoverURL() string {
	return ""
}

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
