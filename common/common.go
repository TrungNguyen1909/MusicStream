package common

type Artist struct {
	Name string `json:"name"`
}
type Album struct {
	Title       string `json:"title"`
	Artist      Artist `json:"artist"`
	Cover       string `json:"cover"`
	CoverSmall  string `json:"cover_small"`
	CoverMedium string `json:"cover_medium"`
	CoverBig    string `json:"cover_big"`
	CoverXL     string `json:"cover_xl"`
}
type Track struct {
	ID       int          `json:"id"`
	Title    string       `json:"title"`
	Artist   Artist       `json:"artist"`
	Album    Album        `json:"album"`
	Duration int          `json:"duration"`
	Lyrics   LyricsResult `json:"lyrics"`
	Rank     int          `json:"rank"`
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
}

type LyricsResult struct {
	RawLyrics    string       `json:"txt"`
	SyncedLyrics []LyricsLine `json:"lrc"`
	Language     string       `json:"lang"`
}
