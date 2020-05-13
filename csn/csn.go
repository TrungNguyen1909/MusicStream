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
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/streamdecoder"
	"github.com/anaskhan96/soup"
)

type csnTrack struct {
	ID       int    `json:"music_id"`
	Title    string `json:"music_title"`
	Artist   string `json:"music_artist"`
	Artists  []string
	Album    string
	Duration int
	Cover    string `json:"music_cover"`
	Link     string `json:"music_link"`
}

//Track represents a track on CSN site
type Track struct {
	csnTrack
	StreamURL string
	playID    string
}

//ID returns the track's ID number on CSN
func (track *Track) ID() string {
	return strconv.Itoa(track.csnTrack.ID)
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
	return track.csnTrack.Duration
}

//ISRC returns the track's ISRC ID
func (track *Track) ISRC() string {
	return ""
}

//CoverURL returns the URL to track's cover art
func (track *Track) CoverURL() string {
	return track.csnTrack.Cover
}

//Download returns a mp3 stream of the track
func (track *Track) Download() (stream io.ReadCloser, err error) {
	if track.StreamURL == "" {
		err = errors.New("Metadata not populated")
		return
	}
	response, err := http.Get(track.StreamURL)
	if err != nil {
		return
	}
	stream = response.Body
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

type csnResult struct {
	Default bool   `json:"default"`
	File    string `json:"file"`
	Label   string `json:"label"`
	Type    string `json:"type"`
}
type csnSearchResult struct {
	Query string `json:"q"`
	Music struct {
		Rows     string     `json:"rows"`
		RowTotal int        `json:"row_total"`
		Page     int        `json:"page"`
		Data     []csnTrack `json:"data"`
	}
}

var pattern *regexp.Regexp
var client *http.Client

//Search takes a query string and returns a slice of matching tracks
func Search(query string) (tracks []common.Track, err error) {
	queryURL, _ := url.Parse("https://chiasenhac.vn/search/real")
	queries := queryURL.Query()
	queries.Add("type", "json")
	queries.Add("rows", "3")
	queries.Add("view_all", "true")
	queries.Add("q", query)
	queryURL.RawQuery = queries.Encode()
	if client == nil {
		cookiesJar, _ := cookiejar.New(nil)
		proxyURL, _ := url.Parse("118.69.50.154:80")
		client = &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}, Jar: cookiesJar, Timeout: 9 * time.Second}
	}
	resp, err := client.Get(queryURL.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var results []csnSearchResult
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return
	}
	result := results[0]
	tracks = make([]common.Track, len(result.Music.Data))
	for i := range result.Music.Data {
		result.Music.Data[i].Artists = strings.Split(result.Music.Data[i].Artist, "; ")
		result.Music.Data[i].Artist = result.Music.Data[i].Artists[0]
	}
	for i, v := range result.Music.Data {
		tracks[i] = &Track{csnTrack: v, playID: common.GenerateID()}
	}
	return
}

//Populate populates the required metadata for downloading the track
func (track *Track) Populate() (err error) {
	if client == nil {
		cookiesJar, _ := cookiejar.New(nil)
		proxyURL, _ := url.Parse("118.69.50.154:80")
		client = &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}, Jar: cookiesJar, Timeout: 9 * time.Second}
	}
	url := track.Link
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	log.Println("csnTrack.Populate: got buf")
	if pattern == nil {
		pattern, _ = regexp.Compile("sources: \\[([^\\]]*)\\]")
	}
	m := pattern.FindSubmatch(buf)
	log.Println("csnTrack.Populate: got match")
	res := bytes.Join([][]byte{[]byte("["), bytes.Trim(m[1], ", \n"), []byte("]")}, []byte(""))
	log.Printf("res: %s\n", res)
	var csnResults []csnResult
	err = json.Unmarshal(res, &csnResults)
	if err != nil {
		return
	}
	log.Println("csnTrack.Populate: got csnResults")
	var streamURL string
	for i := len(csnResults) - 1; i >= 0; i-- {
		if csnResults[i].Type == "mp3" && csnResults[i].File != "" && strings.HasSuffix(csnResults[i].File, ".mp3") {
			streamURL = csnResults[i].File
			break
		}
	}
	if streamURL == "" {
		err = errors.New("no stream URL found")
		return
	}
	log.Println("csnTrack.Populate: got streamURL")
	track.StreamURL = streamURL

	doc := soup.HTMLParse(string(buf))
	log.Println("csnTrack.Populate: got doc parsed")
	list := doc.Find("div", "id", "companion_cover").FindNextElementSibling().Find("h2", "class", "card-title").FindNextElementSibling()
	var album string
	for _, child := range list.Children() {
		span := child.Find("span")
		if span.Pointer == nil {
			continue
		}
		if span.Text() == "Album: " {
			album = child.Find("a").Text()
			break
		}
	}
	track.csnTrack.Album = album
	log.Println("csnTrack.Populate: got album")
	return
}
