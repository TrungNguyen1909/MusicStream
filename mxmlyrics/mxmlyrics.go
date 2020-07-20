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

package mxmlyrics

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/pkg/errors"
)

type mxmResponse struct {
	Message struct {
		Body struct {
			MacroCalls struct {
				TrackLyricsGet struct {
					Message struct {
						Body struct {
							Lyrics struct {
								Explicit         int    `json:"explicit"`
								Instrumental     int    `json:"instrumental"`
								LyricsBody       string `json:"lyrics_body"`
								LyricsCopyright  string `json:"lyrics_copyright"`
								LyricsID         int    `json:"lyrics_id"`
								LyricsLanguage   string `json:"lyrics_language"`
								LyricsTranslated struct {
									LyricsBody       string `json:"lyrics_body"`
									Restricted       int    `json:"restricted"`
									SelectedLanguage string `json:"selected_language"`
								} `json:"lyrics_translated"`
								PublishedStatus int           `json:"published_status"`
								PublisherList   []interface{} `json:"publisher_list"`
								UpdatedTime     string        `json:"updated_time"`
								Verified        int           `json:"verified"`
								WriterList      []interface{} `json:"writer_list"`
							} `json:"lyrics"`
						} `json:"body"`
						Header struct {
							ExecuteTime float64 `json:"execute_time"`
							StatusCode  int     `json:"status_code"`
						} `json:"header"`
					} `json:"message"`
				} `json:"track.lyrics.get"`
				TrackSubtitlesGet struct {
					Message struct {
						Body struct {
							SubtitleList []struct {
								Subtitle struct {
									LyricsCopyright    string `json:"lyrics_copyright"`
									SubtitleBody       string `json:"subtitle_body"`
									SubtitleID         int    `json:"subtitle_id"`
									SubtitleLanguage   string `json:"subtitle_language"`
									SubtitleLength     int    `json:"subtitle_length"`
									SubtitleTranslated struct {
										SelectedLanguage string `json:"selected_language"`
										SubtitleBody     string `json:"subtitle_body"`
									} `json:"subtitle_translated"`
									UpdatedTime string        `json:"updated_time"`
									WriterList  []interface{} `json:"writer_list"`
								} `json:"subtitle"`
							} `json:"subtitle_list"`
						} `json:"body"`
						Header struct {
							Available    int     `json:"available"`
							ExecuteTime  float64 `json:"execute_time"`
							Instrumental int     `json:"instrumental"`
							StatusCode   int     `json:"status_code"`
						} `json:"header"`
					} `json:"message"`
				} `json:"track.subtitles.get"`
			} `json:"macro_calls"`
		} `json:"body"`
		Header struct {
			ExecuteTime      float64       `json:"execute_time"`
			Pid              int           `json:"pid"`
			StatusCode       int           `json:"status_code"`
			SurrogateKeyList []interface{} `json:"surrogate_key_list"`
		} `json:"header"`
	} `json:"message"`
}

//Client represents a MusixMatch lyrics Client
type Client struct {
	httpClient  *http.Client
	userToken   string
	obUserToken string
}

//GetLyrics returns the lyrics of the song with provided information
func (client *Client) GetLyrics(track, artist, album, artists, ISRC, SpotifyURI string, duration int) (result common.LyricsResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("musixmatch.Client.GetLyrics: %v\n", r)
		}
	}()
	rawURL := "http://apic.musixmatch.com/ws/1.1/macro.subtitles.get?format=json&user_language=en&tags=playing&namespace=lyrics_synched&f_subtitle_length_max_deviation=1&subtitle_format=mxm&app_id=mac-ios-v2.0&part=subtitle_translated%2Clyrics_translated&selected_language=en"

	reqURL, _ := url.Parse(rawURL)
	queries := reqURL.Query()
	queries.Add("usertoken", client.userToken)
	if len(client.obUserToken) > 0 {
		queries.Add("OB-USER-TOKEN", client.obUserToken)
	}
	queries.Add("q_track", track)
	queries.Add("q_artist", artist)
	if len(artists) > 0 {
		queries.Add("q_artists", artists)
	} else {
		queries.Add("q_artists", artist)
	}
	queries.Add("q_album", album)
	if duration > 0 {
		queries.Add("q_duration", strconv.Itoa(duration))
		queries.Add("f_subtitle_length", strconv.Itoa(duration))
	}
	if len(SpotifyURI) > 0 {
		queries.Add("track_spotify_id", SpotifyURI)
	}
	reqURL.RawQuery = queries.Encode()
	req, _ := http.NewRequest("GET", reqURL.String(), nil)
	req.Header.Set("Host", "apic.musixmatch.com")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_2) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.4 Safari/605.1.15")
	req.Header.Set("Accept-Language", "en-us")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		defer reader.Close()
	default:
		reader = resp.Body
	}
	var d mxmResponse
	err = json.NewDecoder(reader).Decode(&d)
	if err != nil {
		return
	}
	result = common.LyricsResult{}
	result.RawLyrics = d.Message.Body.MacroCalls.TrackLyricsGet.Message.Body.Lyrics.LyricsBody
	subtitle := d.Message.Body.MacroCalls.TrackSubtitlesGet.Message.Body.SubtitleList[0].Subtitle
	result.Language = subtitle.SubtitleLanguage
	sd := subtitle.SubtitleBody
	if result.Language != "en" && len(subtitle.SubtitleTranslated.SubtitleBody) > 0 {
		st := subtitle.SubtitleTranslated.SubtitleBody
		var subtitleTranslated []common.LyricsLine
		err = json.Unmarshal(([]byte)(st), &subtitleTranslated)
		if err != nil {
			return
		}
		syncedLyrics := make([]common.LyricsLine, len(subtitleTranslated))
		for i, v := range subtitleTranslated {
			syncedLyrics[i].Translated = v.Text
			syncedLyrics[i].Text = v.Original
			syncedLyrics[i].Time = v.Time
		}
		result.SyncedLyrics = syncedLyrics
	}
	var originalSyncedLyrics []common.LyricsLine
	_ = json.Unmarshal(([]byte)(sd), &originalSyncedLyrics)
	if len(result.SyncedLyrics) == 0 {
		result.SyncedLyrics = originalSyncedLyrics
	} else if len(result.SyncedLyrics) == len(originalSyncedLyrics) {
		for i := range result.SyncedLyrics {
			result.SyncedLyrics[i].Original = originalSyncedLyrics[i].Text
		}
	}
	if n := len(result.SyncedLyrics); n > 0 && (result.SyncedLyrics[n-1].Text != "" || result.SyncedLyrics[n-1].Translated != "" || result.SyncedLyrics[n-1].Original != "") {
		addTime := 10.0
		if duration > 0 {
			addTime = float64(duration) - result.SyncedLyrics[n-1].Time.Total
		}
		result.SyncedLyrics = append(result.SyncedLyrics, common.LyricsLine{
			Time: common.LyricsTime{
				Total: result.SyncedLyrics[n-1].Time.Total + addTime,
			},
		})
	}
	return
}

//NewClient returns a new MusixMatch client with provided tokens
func NewClient(MXMUserToken, MXMOBUserToken string) (client *Client, err error) {
	if len(MXMUserToken) <= 0 {
		return nil, errors.WithStack(errors.New("Please provide Musixmatch User Token"))
	}
	cookiesJar, _ := cookiejar.New(nil)
	client = &Client{httpClient: &http.Client{Jar: cookiesJar}, userToken: MXMUserToken, obUserToken: MXMOBUserToken}
	return

}
