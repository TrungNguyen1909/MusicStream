package csn

import (
	"bytes"
	"common"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

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
	response, err := http.DefaultClient.Get(track.StreamURL)
	if err != nil {
		return
	}
	stream = response.Body
	return
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

func Search(query string) (tracks []common.Track, err error) {
	queryURL, _ := url.Parse("https://chiasenhac.vn/search/real")
	queries := queryURL.Query()
	queries.Add("type", "json")
	queries.Add("rows", "3")
	queries.Add("view_all", "true")
	queries.Add("q", query)
	queryURL.RawQuery = queries.Encode()
	resp, err := http.DefaultClient.Get(queryURL.String())
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
	for i, v := range result.Music.Data {
		tracks[i] = Track{csnTrack: v}
	}
	return
}
func (track *Track) Populate() (err error) {
	url := track.Link
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	r, err := regexp.Compile("sources: \\[([^\\]]*)\\]")
	if err != nil {
		return
	}
	m := r.FindSubmatch(buf)
	res := bytes.Join([][]byte{[]byte("["), bytes.Trim(m[1], ", \n"), []byte("]")}, []byte(""))
	var csnResults []csnResult
	err = json.Unmarshal(res, &csnResults)
	if err != nil {
		return
	}
	fmt.Println(csnResults)
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
	doc := soup.HTMLParse(string(buf))
	card := doc.Find("div", "id", "companion_cover").FindNextElementSibling()
	cardTitle := card.Find("h2", "class", "card-title")
	title := cardTitle.Text()
	log.Println(title)
	list := cardTitle.FindNextElementSibling()
	artists := make([]string, 0)
	var album string
	for _, child := range list.Children() {
		span := child.Find("span")
		if span.Pointer == nil {
			continue
		}
		if span.Text() == "Ca sĩ: " {
			for _, artist := range child.FindAll("a") {
				artists = append(artists, artist.Text())
			}
		} else if span.Text() == "Album: " {
			album = child.Find("a").Text()
		}
	}
	track.csnTrack.Artists = artists
	track.csnTrack.Album = album
	track.StreamURL = streamURL
	return
}
