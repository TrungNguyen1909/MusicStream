/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020  Nguyễn Hoàng Trung
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

package youtube

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/rylio/ytdl"
)

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
}

//ID returns the track's ID number on CSN
func (track Track) ID() string {
	return track.ytTrack.ID
}

//Title returns the track's title
func (track Track) Title() string {
	return track.ytTrack.Title
}

//Album returns the track's album title
func (track Track) Album() string {
	return ""
}

//Source returns the track's source
func (track Track) Source() int {
	return common.Youtube
}

//Artist returns the track's main artist
func (track Track) Artist() string {
	return track.ytTrack.ChannelTitle
}

//Artists returns the track's contributors' name, comma-separated
func (track Track) Artists() string {
	return track.ytTrack.ChannelTitle
}

//Duration returns the track's duration
func (track Track) Duration() int {
	return track.ytTrack.Duration
}

//ISRC returns the track's ISRC ID
func (track Track) ISRC() string {
	return ""
}

//CoverURL returns the URL to track's cover art
func (track Track) CoverURL() string {
	return track.ytTrack.CoverURL
}

//Download returns a pcm stream of the track
func (track Track) Download() (stream io.ReadCloser, err error) {
	if track.StreamURL == "" {
		err = errors.New("Metadata not populated")
		return
	}
	response, err := http.Get(track.StreamURL)
	if err != nil {
		return
	}
	stream, err = common.NewWebMDecoder(response.Body)
	return
}

//SpotifyURI returns the track's equivalent spotify song, if known
func (track Track) SpotifyURI() string {
	return ""
}

//PlayID returns a random string which is unique to this instance of Track
func (track Track) PlayID() string {
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

func getLyricsWithLang(id, lang, name string) (result []line, err error) {
	reqURL, _ := url.Parse("https://www.youtube.com/api/timedtext?fmt=srv1")
	queries := reqURL.Query()
	queries.Add("v", id)
	queries.Add("lang", lang)
	queries.Add("name", name)
	reqURL.RawQuery = queries.Encode()
	response, err := http.DefaultClient.Get(reqURL.String())
	if err != nil {
		return
	}
	var t transcript
	err = xml.NewDecoder(response.Body).Decode(&t)
	if err != nil {
		return
	}
	return t.Lines, nil
}

//GetLyrics returns the subtitle for a video id
func GetLyrics(id string) (result common.LyricsResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if ok {
				err = e
			}
			log.Println("Youtube.GetLyrics() panicked: ", err)
		}
	}()
	reqURL, _ := url.Parse("https://video.google.com/timedtext?hl=en&type=list")
	queries := reqURL.Query()
	queries.Add("v", id)
	reqURL.RawQuery = queries.Encode()
	response, err := http.DefaultClient.Get(reqURL.String())
	if err != nil {
		return
	}
	var trl transcriptList
	err = xml.NewDecoder(response.Body).Decode(&trl)
	if err != nil {
		return
	}
	var (
		originalLang       string
		originalLangName   string
		translatedLang     string
		translatedLangName string
	)
	for _, v := range trl.Tracks {
		if v.ID == 0 {
			originalLang = v.LangCode
			originalLangName = v.Name
		}
		if v.LangDefault {
			translatedLang = v.LangCode
			translatedLangName = v.Name
		}
	}
	orig, _ := getLyricsWithLang(id, originalLang, originalLangName)
	trans, _ := getLyricsWithLang(id, translatedLang, translatedLangName)
	log.Printf("Orig: %d - trans: %d", len(orig), len(trans))
	if len(trans) == 0 {
		err = errors.New("No subtitles found")
		return
	}
	result = common.LyricsResult{Language: translatedLang}
	result.SyncedLyrics = make([]common.LyricsLine, len(trans)+1)
	for i, v := range trans {
		result.SyncedLyrics[i].Text = strings.ReplaceAll(html.UnescapeString(v.Text), "\n", " ")
		if len(orig) == len(trans) && originalLang != translatedLang {
			result.SyncedLyrics[i].Translated = strings.ReplaceAll(html.UnescapeString(orig[i].Text), "\n", " ")
		}
		result.SyncedLyrics[i].Time.Total = v.Start
	}
	if len(trans) > 0 {
		result.SyncedLyrics[len(trans)].Time.Total = orig[len(trans)-1].Start + orig[len(trans)-1].Duration
	}
	return
}

//Search finds and returns a track from Youtube with the provided query
func Search(query string) (tracks []common.Track, err error) {
	reqURL, _ := url.Parse("https://www.googleapis.com/youtube/v3/search")
	queries := reqURL.Query()
	queries.Add("key", os.Getenv("YOUTUBE_DEVELOPER_KEY"))
	queries.Add("part", "id,snippet")
	queries.Add("maxResults", "1")
	queries.Add("type", "video")
	queries.Add("q", query)
	reqURL.RawQuery = queries.Encode()
	response, err := http.DefaultClient.Get(reqURL.String())
	if err != nil {
		fmt.Println(err)
		return
	}
	var resp youtubeResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(resp.Items) <= 0 {
		err = errors.New("No track found")
		return
	}
	itrack := Track{
		ytTrack: ytTrack{
			ID:           resp.Items[0].ID.VideoID,
			Title:        html.UnescapeString(resp.Items[0].Snippet.Title),
			ChannelTitle: html.UnescapeString(resp.Items[0].Snippet.ChannelTitle),
			CoverURL:     resp.Items[0].Snippet.Thumbnails.Default.URL,
			Duration:     0,
		},
		playID: common.GenerateID(),
	}
	log.Println(itrack)
	videoInfo, err := ytdl.GetVideoInfoFromID(itrack.ID())
	if err != nil {
		return
	}
	formats := videoInfo.Formats.Extremes(ytdl.FormatAudioBitrateKey, true)
	streamURL, err := videoInfo.GetDownloadURL(formats[0])
	if err != nil {
		return
	}
	itrack.StreamURL = streamURL.String()
	tracks = []common.Track{itrack}
	return
}
