package csn

import (
	"bytes"
	"common"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

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

type Track struct {
	csnTrack
	StreamURL string
	playID    string
}

func (track Track) ID() int {
	return track.csnTrack.ID
}

func (track Track) Title() string {
	return track.csnTrack.Title
}

func (track Track) Album() string {
	return track.csnTrack.Album
}

func (track Track) Source() int {
	return common.CSN
}

func (track Track) Artist() string {
	return track.csnTrack.Artist
}
func (track Track) Artists() string {
	return strings.Join(track.csnTrack.Artists, ", ")
}
func (track Track) Duration() int {
	return track.csnTrack.Duration
}

func (track Track) CoverURL() string {
	return track.csnTrack.Cover
}

func (track Track) Download() (stream io.ReadCloser, err error) {
	if track.StreamURL == "" {
		err = errors.New("Metadata not populated")
		return
	}
	response, err := http.Get(track.StreamURL)
	if err != nil {
		return
	}
	stream = response.Body
	return
}
func (track Track) SpotifyURL() string {
	return ""
}
func (track Track) PlayID() string {
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
		client = &http.Client{Jar: cookiesJar, Timeout: 9 * time.Second}
	}
	resp, err := client.Get(queryURL.String())
	if err != nil {
		return
	}
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
		tracks[i] = Track{csnTrack: v, playID: common.GenerateId()}
	}
	return
}
func (track *Track) Populate() (err error) {
	url := track.Link
	if client == nil {
		cookiesJar, _ := cookiejar.New(nil)
		client = &http.Client{Jar: cookiesJar, Timeout: 9 * time.Second}
	}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
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
		err = errors.New("no stream URL found!")
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
