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

package csn

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/streamdecoder"
	"github.com/pkg/errors"
)

type csnTrack struct {
	ID              json.Number `json:"music_id"`
	Title           string      `json:"music_title"`
	Artist          string      `json:"music_artist"`
	Artists         []string
	Album           string      `json:"music_album"`
	Duration        json.Number `json:"music_length"`
	Cover           string      `json:"music_img"`
	Link            string      `json:"full_url"`
	MusicTitleURL   string      `json:"music_title_url"`
	File320URL      string      `json:"file_320_url"`
	FileLosslessURL string      `json:"file_lossless_url"`
}

type csnMusicInfo struct {
	MusicInfo csnTrack `json:"music_info"`
}

//Track represents a track on CSN site
type Track struct {
	csnTrack
	StreamURL string
	playID    string
	client    *Client
}

//ID returns the track's ID number on CSN
func (track *Track) ID() string {
	return track.csnTrack.ID.String()
}

//Title returns the track's title
func (track *Track) Title() string {
	return track.csnTrack.Title
}

//Album returns the track's album title
func (track *Track) Album() string {
	return track.csnTrack.Album
}

//Source returns the track's source
func (track *Track) Source() int {
	return common.CSN
}

//Artist returns the track's main artist
func (track *Track) Artist() string {
	return track.csnTrack.Artist
}

//Artists returns the track's contributors' name, comma-separated
func (track *Track) Artists() string {
	return strings.Join(track.csnTrack.Artists, ", ")
}

//Duration returns the track's duration
func (track *Track) Duration() int {
	duration, _ := track.csnTrack.Duration.Int64()
	return int(duration)
}

//ISRC returns the track's ISRC ID
func (track *Track) ISRC() string {
	return ""
}

//Href returns the track's link
func (track *Track) Href() string {
	return track.csnTrack.Link
}

//CoverURL returns the URL to track's cover art
func (track *Track) CoverURL() string {
	return track.csnTrack.Cover
}

//Download returns a mp3 stream of the track
func (track *Track) Download() (stream io.ReadCloser, err error) {
	if track.StreamURL == "" {
		err = errors.WithStack(errors.New("Metadata not populated"))
		return
	}
	response, err := http.Get(track.StreamURL)
	if err != nil {
		return
	}
	return response.Body, nil
}

//Stream returns a 16/48 pcm stream of the track
func (track *Track) Stream() (io.ReadCloser, error) {
	stream, err := track.Download()
	if err != nil {
		return nil, err
	}
	stream, err = streamdecoder.NewMP3Decoder(stream)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

//SpotifyURI returns the track's equivalent spotify song, if known
func (track *Track) SpotifyURI() string {
	return ""
}

//PlayID returns a random string which is unique to this instance of Track
func (track *Track) PlayID() string {
	return track.playID
}

type csnSearchResult struct {
	Query string `json:"q"`
	Music struct {
		Rows     int        `json:"rows"`
		RowTotal int        `json:"row_total"`
		Page     int        `json:"page"`
		Data     []csnTrack `json:"data"`
	}
}

//Client represents a CSN client
type Client struct {
	pattern *regexp.Regexp
}

//GetTrackFromURL returns a csn track (if exists) from the provided url
func (client *Client) GetTrackFromURL(q string) (track common.Track, err error) {
	u, err := url.Parse(q)
	if err != nil {
		return nil, errors.WithStack(errors.New("Invalid CSN URL"))
	}
	switch u.Host {
	case "www.chiasenhac.vn", "chiasenhac.vn", "nhacgoc.vn", "vi.chiasenhac.vn":
	default:
		return nil, errors.WithStack(errors.New("Invalid CSN URL"))
	}
	resp, err := http.Get(q)
	if err != nil {
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var musicID string
	m := client.pattern.FindSubmatch(buf)
	if len(m) <= 1 {
		err = errors.WithStack(errors.New("CSN: cannot find music_id from link"))
		return
	}
	musicID = fmt.Sprintf("%s", m[1])
	track = &Track{
		csnTrack: csnTrack{
			ID:   json.Number(musicID),
			Link: q,
		},
		client: client,
	}
	err = track.Populate()
	if err != nil {
		track = nil
		return
	}
	return track, nil
}

//Search takes a query string and returns a slice of matching tracks
func (client *Client) Search(query string) (tracks []common.Track, err error) {
	track, err := client.GetTrackFromURL(query)
	if err == nil {
		return []common.Track{track}, nil
	}
	queryURL, _ := url.Parse("https://chiasenhac.vn/search/real")
	queries := queryURL.Query()
	queries.Add("type", "json")
	queries.Add("rows", "3")
	queries.Add("view_all", "true")
	queries.Add("q", query)
	queryURL.RawQuery = queries.Encode()

	resp, err := http.Get(queryURL.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var results []csnSearchResult
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return
	}
	if len(results) <= 0 {
		return
	}
	result := results[0]
	tracks = make([]common.Track, len(result.Music.Data))
	for i := range result.Music.Data {
		result.Music.Data[i].Artists = strings.Split(result.Music.Data[i].Artist, "; ")
		result.Music.Data[i].Artist = result.Music.Data[i].Artists[0]
	}
	for i, v := range result.Music.Data {
		tracks[i] = &Track{csnTrack: v, playID: common.GenerateID(), client: client}
	}
	return
}

//Populate populates the required metadata for downloading the track
func (track *Track) Populate() (err error) {
	queryURL, _ := url.Parse("http://old.chiasenhac.vn/api/listen.php?code=csn22052018&return=json")
	queries := queryURL.Query()
	queries.Add("m", track.csnTrack.ID.String())
	// queries.Add("url", track.csnTrack.MusicTitleURL)
	queryURL.RawQuery = queries.Encode()
	resp, err := http.Get(queryURL.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var result csnMusicInfo
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return
	}
	track.csnTrack = result.MusicInfo
	track.StreamURL = track.csnTrack.File320URL
	track.csnTrack.Artists = strings.Split(track.csnTrack.Artist, "; ")
	track.csnTrack.Artist = track.csnTrack.Artists[0]
	return
}

//NewClient returns a new CSN Client
func NewClient() (*Client, error) {
	pattern, _ := regexp.Compile(`loadPlayList\((\d+)\)`)
	return &Client{pattern: pattern}, nil
}
