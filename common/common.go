/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020 Nguyễn Hoàng Trung(TrungNguyen1909)
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"io"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type StreamFormat int

const (
	RawStream = iota
	FFmpegStream
)

//Stream is an encoded audio stream
type Stream interface {
	Format() int
	Body() io.ReadCloser
}

//Track represents a track from any sources
type Track interface {
	ID() string
	IsRadio() bool
	Title() string
	Artist() string
	Artists() string
	Album() string
	ISRC() string
	Href() string
	CoverURL() string
	Duration() int
	SpotifyURI() string
	PlayID() string
	Populate() error
	Stream() (Stream, error)
}

//TrackWithLyrics is a track that is responsible for fetching its own lyrics
type TrackWithLyrics interface {
	Track
	GetLyrics() (LyricsResult, error)
}

//TrackMetadata contains essential informations about a track for client
type TrackMetadata struct {
	Title      string       `json:"title"`
	IsRadio    bool         `json:"is_radio"`
	Duration   int          `json:"duration"`
	Artist     string       `json:"artist"`
	Artists    string       `json:"artists"`
	Album      string       `json:"album"`
	CoverURL   string       `json:"cover"`
	Lyrics     LyricsResult `json:"lyrics"`
	PlayID     string       `json:"playId"`
	SpotifyURI string       `json:"spotifyURI"`
	ID         string       `json:"id"`
	Href       string       `json:"href"`
}

//GetMetadata returns a new TrackMetadata created from a provided Track
func GetMetadata(track Track) (d TrackMetadata) {
	d = TrackMetadata{}
	d.Title = track.Title()
	d.IsRadio = track.IsRadio()
	d.Duration = track.Duration()
	d.Artist = track.Artist()
	d.Artists = track.Artists()
	d.Album = track.Album()
	d.CoverURL = track.CoverURL()
	d.PlayID = track.PlayID()
	d.ID = track.ID()
	d.SpotifyURI = track.SpotifyURI()
	d.Href = track.Href()
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

//GenerateID generates a random uuid
func GenerateID() (id string) {
	uuid := uuid.New()
	return uuid.String()
}

//DefaultTrack represents the metadata will be shown when nothing is playing
type DefaultTrack struct{}

func (track *DefaultTrack) ID() string {
	return "0"
}

func (track *DefaultTrack) IsRadio() bool {
	return true
}

func (track *DefaultTrack) Title() string {
	return "Idle..."
}

func (track *DefaultTrack) Artist() string {
	return "Please enqueue"
}

func (track *DefaultTrack) Artists() string {
	return "Please enqueue"
}

func (track *DefaultTrack) Album() string {
	return ""
}

func (track *DefaultTrack) ISRC() string {
	return ""
}
func (track *DefaultTrack) Href() string {
	return ""
}
func (track *DefaultTrack) CoverURL() string {
	return ""
}

func (track *DefaultTrack) Duration() int {
	return 0
}

func (track *DefaultTrack) SpotifyURI() string {
	return ""
}

func (track *DefaultTrack) PlayID() string {
	return ""
}

func (track *DefaultTrack) Populate() error {
	return errors.WithStack(errors.New("not implemented"))
}

func (track *DefaultTrack) Download() (io.ReadCloser, error) {
	return nil, errors.WithStack(errors.New("not implemented"))
}

//Stream is intentionally not implemented on this track type
func (track *DefaultTrack) Stream() (Stream, error) {
	return nil, errors.WithStack(errors.New("not implemented"))
}

//MusicSource is an interface for a music source
type MusicSource interface {
	Search(query string) ([]Track, error)
	Name() string
	DisplayName() string
}

//MusicSourceInfo contains information about a music source
type MusicSourceInfo struct {
	//Name is the full name of the source
	Name string `json:"name"`
	//DisplayName is the shortened name of the source, used to display on search bar
	DisplayName string `json:"display_name"`
	//ID is the source's id, assigned by the server, used for querying tracks
	ID int `json:"id"`
}

func GetMusicSourceInfo(s MusicSource) MusicSourceInfo {
	return MusicSourceInfo{Name: s.Name(), DisplayName: s.DisplayName()}
}
