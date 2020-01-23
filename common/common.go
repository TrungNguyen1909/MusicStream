package common

import "strings"

type Artist struct {
	Name string `json:"name"`
}
type Album struct {
	Title       string `json:"title"`
	Cover       string `json:"cover"`
	CoverSmall  string `json:"cover_small"`
	CoverMedium string `json:"cover_medium"`
	CoverBig    string `json:"cover_big"`
	CoverXL     string `json:"cover_xl"`
}
type Track struct {
	ID           int          `json:"id"`
	Title        string       `json:"title"`
	Artist       Artist       `json:"artist"`
	Artists      string       `json:"artists"`
	Contributors []Artist     `json:"contributors"`
	Album        Album        `json:"album"`
	Duration     int          `json:"duration"`
	Lyrics       LyricsResult `json:"lyrics"`
	Rank         int          `json:"rank"`
}

func (track *Track) GetArtists() (artists string) {
	for _, v := range track.Contributors {
		artists = strings.Join([]string{artists, v.Name}, ", ")
	}
	artists = artists[2:]
	track.Artists = artists
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
