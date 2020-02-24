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

package spotify

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

//Client represents a Spotify Client
type Client struct {
	AccessToken                      string `json:"accessToken"`
	AccessTokenExpirationTimestampMs int64  `json:"accessTokenExpirationTimestampMs"`
	ClientID                         string `json:"clientId"`
	IsAnonymous                      bool   `json:"isAnonymous"`
}
type searchResponse struct {
	BestMatch struct {
		Items []struct {
			Album struct {
				AlbumType string `json:"album_type"`
				Artists   []struct {
					ExternalUrls struct {
						Spotify string `json:"spotify"`
					} `json:"external_urls"`
					Href string `json:"href"`
					ID   string `json:"id"`
					Name string `json:"name"`
					Type string `json:"type"`
					URI  string `json:"uri"`
				} `json:"artists"`
				ExternalUrls struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href   string `json:"href"`
				ID     string `json:"id"`
				Images []struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"images"`
				Name                 string `json:"name"`
				ReleaseDate          string `json:"release_date"`
				ReleaseDatePrecision string `json:"release_date_precision"`
				TotalTracks          int    `json:"total_tracks"`
				Type                 string `json:"type"`
				URI                  string `json:"uri"`
			} `json:"album"`
			Artists []struct {
				ExternalUrls struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href string `json:"href"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
				URI  string `json:"uri"`
			} `json:"artists"`
			DiscNumber  int  `json:"disc_number"`
			DurationMs  int  `json:"duration_ms"`
			Explicit    bool `json:"explicit"`
			ExternalIds struct {
				Isrc string `json:"isrc"`
			} `json:"external_ids"`
			ExternalUrls struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			Href        string `json:"href"`
			ID          string `json:"id"`
			IsLocal     bool   `json:"is_local"`
			IsPlayable  bool   `json:"is_playable"`
			Name        string `json:"name"`
			Popularity  int    `json:"popularity"`
			PreviewURL  string `json:"preview_url"`
			TrackNumber int    `json:"track_number"`
			Type        string `json:"type"`
			URI         string `json:"uri"`
		} `json:"items"`
	} `json:"best_match"`
	Tracks struct {
		Href  string `json:"href"`
		Items []struct {
			Album struct {
				AlbumType string `json:"album_type"`
				Artists   []struct {
					ExternalUrls struct {
						Spotify string `json:"spotify"`
					} `json:"external_urls"`
					Href string `json:"href"`
					ID   string `json:"id"`
					Name string `json:"name"`
					Type string `json:"type"`
					URI  string `json:"uri"`
				} `json:"artists"`
				ExternalUrls struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href   string `json:"href"`
				ID     string `json:"id"`
				Images []struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"images"`
				Name                 string `json:"name"`
				ReleaseDate          string `json:"release_date"`
				ReleaseDatePrecision string `json:"release_date_precision"`
				TotalTracks          int    `json:"total_tracks"`
				Type                 string `json:"type"`
				URI                  string `json:"uri"`
			} `json:"album"`
			Artists []struct {
				ExternalUrls struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href string `json:"href"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
				URI  string `json:"uri"`
			} `json:"artists"`
			DiscNumber  int  `json:"disc_number"`
			DurationMs  int  `json:"duration_ms"`
			Explicit    bool `json:"explicit"`
			ExternalIds struct {
				Isrc string `json:"isrc"`
			} `json:"external_ids"`
			ExternalUrls struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			Href        string `json:"href"`
			ID          string `json:"id"`
			IsLocal     bool   `json:"is_local"`
			IsPlayable  bool   `json:"is_playable"`
			Name        string `json:"name"`
			Popularity  int    `json:"popularity"`
			PreviewURL  string `json:"preview_url"`
			TrackNumber int    `json:"track_number"`
			Type        string `json:"type"`
			URI         string `json:"uri"`
		} `json:"items"`
		Limit    int         `json:"limit"`
		Next     string      `json:"next"`
		Offset   int         `json:"offset"`
		Previous interface{} `json:"previous"`
		Total    int         `json:"total"`
	} `json:"tracks"`
}

func (client *Client) fetchToken() (err error) {
	if time.Now().Before(time.Unix(0, client.AccessTokenExpirationTimestampMs*1000000)) {
		return
	}
	resp, err := http.DefaultClient.Get("https://open.spotify.com/access_token")
	if err != nil {
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&client)
	return
}

//SearchTrackQuery returns a Spotify track with provided query
func (client *Client) SearchTrackQuery(query string) (sTrack, sArtist, sAlbum, sISRC, sURI string, err error) {
	client.fetchToken()
	reqURL, _ := url.Parse("https://api.spotify.com/v1/search?type=track&decorate_restrictions=false&best_match=false&limit=3&userless=true&market=VN")
	queries := reqURL.Query()
	queries.Add("q", query)
	reqURL.RawQuery = queries.Encode()
	req, _ := http.NewRequest("GET", reqURL.String(), nil)
	req.Header.Add("Authorization", "Bearer "+client.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	var d searchResponse
	err = json.NewDecoder(resp.Body).Decode(&d)
	if err != nil {
		return
	}
	if len(d.Tracks.Items) <= 0 {
		err = errors.New("No Spotify track found")
		return
	}
	sTrack = d.Tracks.Items[0].Name
	sArtist = d.Tracks.Items[0].Artists[0].Name
	sAlbum = d.Tracks.Items[0].Album.Name
	sISRC = d.Tracks.Items[0].ExternalIds.Isrc
	sURI = d.Tracks.Items[0].URI
	return
}

//SearchTrack returns a Spotify track with provided fields
func (client *Client) SearchTrack(track, artist, album, isrc string) (sTrack, sArtist, sAlbum, sISRC, sURI string, err error) {
	client.fetchToken()
	reqURL, _ := url.Parse("https://api.spotify.com/v1/search?type=track&decorate_restrictions=false&best_match=true&limit=3&userless=true&market=VN")
	queries := reqURL.Query()
	var query string
	if len(track) > 0 {
		query += "track:\"" + track + "\""
	}
	if len(artist) > 0 {
		query += "artist:\"" + artist + "\""
	}
	if len(album) > 0 {
		query += "album:\"" + album + "\""
	}
	if len(isrc) > 0 {
		query += "isrc:" + isrc
	}
	queries.Add("q", query)
	reqURL.RawQuery = queries.Encode()
	req, _ := http.NewRequest("GET", reqURL.String(), nil)
	req.Header.Add("Authorization", "Bearer "+client.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	var d searchResponse
	err = json.NewDecoder(resp.Body).Decode(&d)
	if err != nil {
		return
	}
	if len(d.Tracks.Items) <= 0 {
		err = errors.New("No Spotify track found")
		return
	}
	sTrack = d.Tracks.Items[0].Name
	sArtist = d.Tracks.Items[0].Artists[0].Name
	sAlbum = d.Tracks.Items[0].Album.Name
	sISRC = d.Tracks.Items[0].ExternalIds.Isrc
	sURI = d.Tracks.Items[0].URI
	return
}

//NewClient returns new Spotify Client
func NewClient() (client *Client, err error) {
	client = &Client{}
	err = client.fetchToken()
	return
}
