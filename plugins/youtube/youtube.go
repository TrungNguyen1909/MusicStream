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

package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/kkdai/youtube/v2"
	"github.com/pkg/errors"
)

var Name string = "Youtube"
var DisplayName string = "YT"

type youtubeStream struct {
	body io.ReadCloser
}

func (s *youtubeStream) Format() int {
	return common.WebMStream
}
func (s *youtubeStream) Body() io.ReadCloser {
	return s.body
}

type youtubeResponse struct {
	Etag  string `json:"etag"`
	Items []struct {
		Etag string `json:"etag"`
		ID   struct {
			Kind    string `json:"kind"`
			VideoID string `json:"videoId"`
		} `json:"id"`
		Kind    string `json:"kind"`
		Snippet struct {
			ChannelID            string `json:"channelId"`
			ChannelTitle         string `json:"channelTitle"`
			Description          string `json:"description"`
			LiveBroadcastContent string `json:"liveBroadcastContent"`
			PublishedAt          string `json:"publishedAt"`
			Thumbnails           struct {
				Default struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"default"`
				High struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"high"`
				Medium struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"medium"`
			} `json:"thumbnails"`
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
	Kind          string `json:"kind"`
	NextPageToken string `json:"nextPageToken"`
	PageInfo      struct {
		ResultsPerPage int `json:"resultsPerPage"`
		TotalResults   int `json:"totalResults"`
	} `json:"pageInfo"`
	RegionCode string `json:"regionCode"`
}

//Track represents a Youtube Video
type Track struct {
	ytTrack
	playID    string
	StreamURL string
}
type ytTrack struct {
	ID           string
	Title        string
	ChannelTitle string
	CoverURL     string
	Duration     int
	Video        *youtube.Video
}

//ID returns the track's ID number on CSN
func (track *Track) ID() string {
	return track.ytTrack.ID
}

//Title returns the track's title
func (track *Track) Title() string {
	return track.ytTrack.Title
}

//Album returns the track's album title
func (track *Track) Album() string {
	return ""
}

func (track *Track) IsRadio() bool {
	return false
}

//Artist returns the track's main artist
func (track *Track) Artist() string {
	return track.ytTrack.ChannelTitle
}

//Artists returns the track's contributors' name, comma-separated
func (track *Track) Artists() string {
	return track.ytTrack.ChannelTitle
}

//Duration returns the track's duration
func (track *Track) Duration() int {
	return track.ytTrack.Duration
}

//ISRC returns the track's ISRC ID
func (track *Track) ISRC() string {
	return ""
}

//Href returns the track's link
func (track *Track) Href() string {
	return fmt.Sprintf("https://youtu.be/%s", track.ID())
}

//CoverURL returns the URL to track's cover art
func (track *Track) CoverURL() string {
	return track.ytTrack.CoverURL
}

//Download returns a webm stream of the track
func (track *Track) Download() (stream io.ReadCloser, err error) {
	if track.Video == nil {
		err = errors.WithStack(errors.New("Metadata not populated"))
		return
	}
	c := youtube.Client{}
	response, err := c.GetStream(track.ytTrack.Video, track.ytTrack.Video.Formats.FindByItag(251))
	if err != nil {
		return
	}
	return response.Body, nil
}

//Stream returns a 16/48 pcm stream of the track
func (track *Track) Stream() (common.Stream, error) {
	stream, err := track.Download()
	if err != nil {
		return nil, err
	}
	return &youtubeStream{body: stream}, nil
}

//Populate populates metadata for Download
func (track *Track) Populate() (err error) {
	c := youtube.Client{}
	track.Video, err = c.GetVideo(track.ytTrack.ID)
	if err != nil {
		return
	}
	track.ytTrack.Duration = int(track.Video.Duration.Seconds())
	return
}

//SpotifyURI returns the track's equivalent spotify song, if known
func (track *Track) SpotifyURI() string {
	return ""
}

//PlayID returns a random string which is unique to this instance of Track
func (track *Track) PlayID() string {
	return track.playID
}

type transcriptList struct {
	XMLName xml.Name          `xml:"transcript_list"`
	DocID   string            `xml:"docid,attr"`
	Tracks  []transcriptTrack `xml:"track"`
}
type transcriptTrack struct {
	ID             int    `xml:"id,attr"`
	Name           string `xml:"name,attr"`
	LangCode       string `xml:"lang_code,attr"`
	LangOriginal   string `xml:"lang_original,attr"`
	LangTranslated string `xml:"lang_translated,attr"`
	LangDefault    bool   `xml:"lang_default,attr"`
}
type transcript struct {
	XMLName xml.Name `xml:"transcript"`
	Lines   []line   `xml:"text"`
}
type line struct {
	Start    float64 `xml:"start,attr"`
	Duration float64 `xml:"dur,attr"`
	Text     string  `xml:",chardata"`
}

//Client represents a Youtube client
type Client struct {
	apiKey string
}

func (track *Track) getLyricsWithLang(lang, name string) (result []line, err error) {
	if len(track.ID()) == 0 || len(lang) == 0 {
		return nil, errors.WithStack(errors.New("Invalid Arguments"))
	}
	reqURL, _ := url.Parse("https://www.youtube.com/api/timedtext?fmt=srv1")
	queries := reqURL.Query()
	queries.Add("v", track.ID())
	queries.Add("lang", lang)
	queries.Add("name", name)
	reqURL.RawQuery = queries.Encode()
	response, err := http.DefaultClient.Get(reqURL.String())
	if err != nil {
		return
	}
	defer response.Body.Close()
	var t transcript
	err = xml.NewDecoder(response.Body).Decode(&t)
	if err != nil {
		return
	}
	return t.Lines, nil
}

func (client *Client) Name() string {
	return Name
}
func (client *Client) DisplayName() string {
	return DisplayName
}

//GetLyrics returns the subtitle for a video id
func (track *Track) GetLyrics() (result common.LyricsResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if ok {
				err = e
			}
			log.Println("[Youtube] GetLyrics() panicked: ", err)
		}
	}()
	reqURL, _ := url.Parse("https://video.google.com/timedtext?hl=en&type=list")
	queries := reqURL.Query()
	queries.Add("v", track.ID())
	reqURL.RawQuery = queries.Encode()
	response, err := http.DefaultClient.Get(reqURL.String())
	if err != nil {
		return
	}
	defer response.Body.Close()
	var trl transcriptList
	err = xml.NewDecoder(response.Body).Decode(&trl)
	if err != nil {
		return
	}
	var (
		defaultLang        string
		defaultLangName    string
		translatedLang     string
		translatedLangName string
	)
	for _, v := range trl.Tracks {
		if v.LangDefault {
			defaultLang = v.LangCode
			defaultLangName = v.Name
			break
		}
	}
	if !strings.HasPrefix(defaultLang, "en") {
		for _, v := range trl.Tracks {
			if strings.HasPrefix(v.LangCode, "en") {
				translatedLang = v.LangCode
				translatedLangName = v.Name
				break
			}
		}
	}
	def, _ := track.getLyricsWithLang(defaultLang, defaultLangName)
	trans, _ := track.getLyricsWithLang(translatedLang, translatedLangName)
	if len(def) == 0 {
		return
	}
	result = common.LyricsResult{Language: defaultLang}
	result.SyncedLyrics = make([]common.LyricsLine, len(def)+1)
	for i, v := range def {
		result.SyncedLyrics[i].Text = strings.ReplaceAll(html.UnescapeString(v.Text), "\n", " ")
		if len(trans) == len(def) && v.Start == trans[i].Start {
			result.SyncedLyrics[i].Text = strings.ReplaceAll(html.UnescapeString(trans[i].Text), "\n", " ")
		}
		result.SyncedLyrics[i].Time.Total = v.Start
	}
	if len(def) > 0 {
		result.SyncedLyrics[len(def)].Time.Total = def[len(def)-1].Start + def[len(def)-1].Duration
	}
	return
}
func (client *Client) extractVideoID(q string) (videoID string, err error) {
	u, err := url.Parse(q)
	if err != nil {
		return "", err
	}
	switch u.Host {
	case "www.youtube.com", "youtube.com":
		if u.Path == "/watch" {
			return u.Query().Get("v"), nil
		}
		if strings.HasPrefix(u.Path, "/embed/") {
			return u.Path[7:], nil
		}
	case "youtu.be":
		if len(u.Path) > 1 {
			return u.Path[1:], nil
		}
	}
	return "", nil
}

//GetTrackFromVideoID returns a track on Youtube with provided videoID
func (client *Client) GetTrackFromVideoID(videoID string) (track common.Track, err error) {
	c := youtube.Client{}
	videoInfo, err := c.GetVideo("youtu.be/" + videoID)
	if err != nil {
		return
	}
	itrack := &Track{
		ytTrack: ytTrack{
			ID:           videoInfo.ID,
			Title:        html.UnescapeString(videoInfo.Title),
			ChannelTitle: html.UnescapeString(videoInfo.Author),
			Duration:     int(videoInfo.Duration.Seconds()),
			CoverURL:     fmt.Sprintf("https://img.youtube.com/vi/%s/sddefault.jpg", videoInfo.ID),
		},
		playID: common.GenerateID(),
	}
	if err != nil {
		return
	}
	track = itrack
	return
}

//Search finds and returns a list of tracks from Youtube with the provided query
func (client *Client) Search(query string) (tracks []common.Track, err error) {
	videoID, err := client.extractVideoID(query)
	if err == nil && len(videoID) > 0 {
		track, err := client.GetTrackFromVideoID(videoID)
		if err == nil && track != nil {
			return []common.Track{track}, nil
		}
	}
	reqURL, _ := url.Parse("https://www.googleapis.com/youtube/v3/search")
	queries := reqURL.Query()
	queries.Add("key", client.apiKey)
	queries.Add("part", "id,snippet")
	queries.Add("maxResults", "1")
	queries.Add("type", "video")
	queries.Add("q", query)
	reqURL.RawQuery = queries.Encode()
	response, err := http.DefaultClient.Get(reqURL.String())
	if err != nil {
		return
	}
	defer response.Body.Close()
	var resp youtubeResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil {
		return
	}
	if len(resp.Items) <= 0 {
		return
	}
	itracks := make([]common.Track, len(resp.Items))
	for i, item := range resp.Items {
		itrack := &Track{
			ytTrack: ytTrack{
				ID:           item.ID.VideoID,
				Title:        html.UnescapeString(item.Snippet.Title),
				ChannelTitle: html.UnescapeString(item.Snippet.ChannelTitle),
				CoverURL:     item.Snippet.Thumbnails.High.URL,
				Duration:     0,
			},
			playID: common.GenerateID(),
		}
		itracks[i] = itrack
	}
	tracks = itracks
	return
}

//NewClient returns a new Client with the provided Youtube Developer API key
func NewClient() (client common.MusicSource, err error) {
	DeveloperAPIKey := os.Getenv("YOUTUBE_DEVELOPER_KEY")
	if len(DeveloperAPIKey) <= 0 {
		return nil, errors.WithStack(errors.New("Please provide Youtube Data API v3 key"))
	}
	client = &Client{apiKey: DeveloperAPIKey}
	return
}
