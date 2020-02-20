package common

import (
	"crypto/rand"
	"io"
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
