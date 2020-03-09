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

package deezer

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/TrungNguyen1909/MusicStream/spotify"
	"github.com/TrungNguyen1909/MusicStream/streamdecoder"

	"github.com/TrungNguyen1909/MusicStream/common"

	"golang.org/x/crypto/blowfish"
	"golang.org/x/text/encoding/charmap"
)

const (
	deezerURL        = "https://www.deezer.com"
	ajaxActionURL    = "https://www.deezer.com/ajax/action.php"
	unofficialAPIURL = "https://www.deezer.com/ajax/gw-light.php"
	trackQualityID   = 3
)

//Artist represents an artist on Deezer
type Artist struct {
	Name string `json:"name"`
}

//Album represents an album on Deezer
type Album struct {
	Title       string `json:"title"`
	Cover       string `json:"cover"`
	CoverSmall  string `json:"cover_small"`
	CoverMedium string `json:"cover_medium"`
	CoverBig    string `json:"cover_big"`
	CoverXL     string `json:"cover_xl"`
}

//Track represents a track on Deezer
type Track struct {
	deezerTrack
	StreamURL   string
	BlowfishKey []byte
	playID      string
}

//ID returns the track's ID number on Deezer
func (track *Track) ID() string {
	return strconv.Itoa(track.deezerTrack.ID)
}

//Title returns the track's title
func (track *Track) Title() string {
	return track.deezerTrack.Title
}

//Album returns the track's album title
func (track *Track) Album() string {
	return track.deezerTrack.Album.Title
}

//Source returns the track's source
func (track *Track) Source() int {
	return common.Deezer
}

//Artist returns the track's main artist
func (track *Track) Artist() string {
	return track.deezerTrack.Artist.Name
}

//Artists returns the track's contributors' name, comma-separated
func (track *Track) Artists() string {
	artists := ""
	if len(track.deezerTrack.Contributors) > 0 {
		for _, v := range track.deezerTrack.Contributors {
			artists = strings.Join([]string{artists, v.Name}, ", ")
		}
		artists = artists[2:]
	} else {
		artists = track.deezerTrack.Artist.Name
	}
	return artists
}

//Duration returns the track's duration
func (track *Track) Duration() int {
	return track.deezerTrack.Duration
}

//ISRC returns the track's ISRC ID
func (track *Track) ISRC() string {
	return track.deezerTrack.ISRC
}

//CoverURL returns the URL to track's cover art
func (track *Track) CoverURL() string {
	return track.deezerTrack.Album.CoverXL
}

//Download returns a mp3 stream of the track
func (track *Track) Download() (stream io.ReadCloser, err error) {
	if track.StreamURL == "" || len(track.BlowfishKey) == 0 {
		err = errors.New("Metadata not yet populated")
		return
	}
	req := track.client.makeRequest("GET", track.StreamURL, []byte(""))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	if response.StatusCode != http.StatusOK {
		err = errors.New(fmt.Sprint("deezerTrack Download failed: ", track.StreamURL, " ", response.Status))
		return
	}
	stream, err = streamdecoder.NewMP3Decoder(&trackDecrypter{r: response.Body, BlowfishKey: track.BlowfishKey})
	if err != nil {
		return
	}
	return
}

//Populate populates track metadata for Download
func (track *Track) Populate() (err error) {
	return track.client.PopulateMetadata(track)
}

//SpotifyURI returns the track's equivalent spotify song, if known
func (track *Track) SpotifyURI() string {
	return track.deezerTrack.SpotifyURI
}

//PlayID returns a random string which is unique to this instance of Track
func (track *Track) PlayID() string {
	return track.playID
}

//SetSpotifyURI set the track's SpotifyURI with the provided one
func (track *Track) SetSpotifyURI(sURI string) {
	track.deezerTrack.SpotifyURI = sURI
}

type getUserDataResults struct {
	CheckForm string `json:"checkForm"`
}
type getUserDataResponse struct {
	Error   []interface{}      `json:"error"`
	Results getUserDataResults `json:"results"`
}
type pageTrackData struct {
	MD5Origin    string `json:"MD5_ORIGIN"`
	SNGId        string `json:"SNG_ID"`
	MediaVersion string `json:"MEDIA_VERSION"`
}
type pageTrackResults struct {
	Data pageTrackData `json:"DATA"`
}
type pageTrackResponse struct {
	Error   []interface{}    `json:"error"`
	Results pageTrackResults `json:"results"`
}
type searchTrackResponse struct {
	Data []deezerTrack `json:"data"`
}

type deezerTrack struct {
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	Artist       Artist   `json:"artist"`
	Artists      string   `json:"artists"`
	Contributors []Artist `json:"contributors"`
	Album        Album    `json:"album"`
	Duration     int      `json:"duration"`
	Rank         int      `json:"rank"`
	ISRC         string   `json:"isrc"`
	SpotifyURI   string
	client       *Client
}

type trackDecrypter struct {
	r           io.ReadCloser
	BlowfishKey []byte
	counter     int
	byteCounter int
	buffer      bytes.Buffer
	ended       bool
}

func (decrypter *trackDecrypter) createCipher() cipher.BlockMode {

	blowfishEngine, err := blowfish.NewCipher(decrypter.BlowfishKey)
	if err != nil {
		log.Panic(err)
	}
	blowfishCBC := cipher.NewCBCDecrypter(blowfishEngine, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07})
	return blowfishCBC
}
func (decrypter *trackDecrypter) decrypt(n int) {
	for decrypter.buffer.Len() < n && !decrypter.ended {
		buf := make([]byte, 2048)
		size, err := decrypter.r.Read(buf)
		if decrypter.counter%3 == 0 && size == 2048 {
			blowfish := decrypter.createCipher()
			blowfish.CryptBlocks(buf, buf)
		}
		decrypter.counter++
		decrypter.buffer.Write(buf)
		if err != nil {
			decrypter.ended = true
			break
		}
	}
}
func (decrypter *trackDecrypter) Read(p []byte) (n int, err error) {
	decrypter.decrypt(len(p))
	n, err = decrypter.buffer.Read(p)
	if err != nil || decrypter.ended {
		err = io.EOF
	}
	return n, err
}
func (decrypter *trackDecrypter) Close() error {
	return decrypter.r.Close()
}

//Client represents a Deezer client
type Client struct {
	httpHeaders        http.Header
	arlCookie          *http.Cookie
	httpClient         *http.Client
	ajaxActionURL      *url.URL
	unofficialAPIURL   *url.URL
	unofficialAPIQuery url.Values
	deezerURL          *url.URL
	spotifyClient      *spotify.Client
}

//NewClient returns a new Deezer Client
func NewClient() (client *Client) {
	client = &Client{}
	cookiesJar, _ := cookiejar.New(nil)
	client.httpClient = &http.Client{Jar: cookiesJar}
	client.httpHeaders = http.Header{}
	client.unofficialAPIQuery = make(url.Values)
	client.httpHeaders.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/72.0.3626.121 Safari/537.36")
	client.httpHeaders.Set("cache-control", "max-age=0")
	client.httpHeaders.Set("accept-language", "en-US,en;q=0.9,en-US;q=0.8,en;q=0.7")
	client.httpHeaders.Set("accept-charset", "utf-8,ISO-8859-1;q=0.8,*;q=0.7")
	client.httpHeaders.Set("content-type", "text/plain;charset=UTF-8")
	client.httpHeaders.Set("cookie", "arl="+os.Getenv("DEEZER_ARL"))
	client.ajaxActionURL, _ = url.Parse(ajaxActionURL)
	client.unofficialAPIURL, _ = url.Parse(unofficialAPIURL)
	client.unofficialAPIQuery.Set("api_version", "1.0")
	client.unofficialAPIQuery.Set("input", "3")
	client.unofficialAPIQuery.Set("api_token", "")
	client.deezerURL, _ = url.Parse(deezerURL)
	spotifyClient, err := spotify.NewClient()
	if err != nil {
		log.Printf("spotify.NewClient() failed: %v\n", err)
	} else {
		client.spotifyClient = spotifyClient
	}
	client.initDeezerAPI()
	return
}

func getAPICID() string {
	return strconv.Itoa(int(math.Floor(rand.Float64() * 1e9)))
}
func (client *Client) cleanupCookieJar() {
	cookiesJar, _ := cookiejar.New(nil)
	client.httpClient.Jar = cookiesJar
}
func (client *Client) makeRequest(method, url string, body []byte) *http.Request {
	request, _ := http.NewRequest(method, url, bytes.NewReader(body))
	request.Header = client.httpHeaders
	return request
}
func (client *Client) makeUnofficialAPIRequest(method string, body []byte) *http.Request {
	client.unofficialAPIQuery.Set("method", method)
	client.unofficialAPIQuery.Set("cid", getAPICID())
	client.unofficialAPIURL.RawQuery = client.unofficialAPIQuery.Encode()
	return client.makeRequest("POST", client.unofficialAPIURL.String(), body)
}
func (client *Client) initDeezerAPI() {
	client.unofficialAPIQuery.Set("api_token", "")
	request := client.makeUnofficialAPIRequest("deezer.getUserData", []byte(""))
	client.cleanupCookieJar()
	response, err := client.httpClient.Do(request)
	if err != nil {
		log.Println("deezer.initDeezerAPI() failed: ", err)
		return
	}
	defer response.Body.Close()
	var resp getUserDataResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil || len(resp.Results.CheckForm) <= 0 {
		return
	}
	client.unofficialAPIQuery.Set("api_token", resp.Results.CheckForm)
	log.Printf("Successfully initiated Deezer API. Checkform: \"%s\"\n", resp.Results.CheckForm)
}
func (client *Client) getTrackInfo(trackID int, secondTry bool) (trackInfo pageTrackData, err error) {
	data := map[string]interface{}{
		"SNG_ID": trackID,
	}
	encoded, _ := json.Marshal(data)
	request := client.makeUnofficialAPIRequest("deezer.pageTrack", encoded)
	response, err := client.httpClient.Do(request)
	var resp pageTrackResponse
	if err == nil {
		defer response.Body.Close()
		err = json.NewDecoder(response.Body).Decode(&resp)
	}
	if err != nil || len(resp.Results.Data.MD5Origin) <= 0 {
		if secondTry {
			if err != nil {
				err = errors.New("Failed to get trackInfo adequately " + err.Error())
			} else {
				err = errors.New("Failed to get trackInfo adequately")
			}
			return
		}
		client.initDeezerAPI()
		return client.getTrackInfo(trackID, true)
	}
	return resp.Results.Data, nil
}
func (client *Client) getSongFileName(trackInfo pageTrackData) string {
	encoder := charmap.Windows1252.NewEncoder()
	step1 := strings.Join([]string{trackInfo.MD5Origin, strconv.Itoa(trackQualityID), trackInfo.SNGId, trackInfo.MediaVersion}, "¤")
	step1encoded, _ := encoder.Bytes([]byte(step1))
	step2 := fmt.Sprintf("%x¤%s¤", md5.Sum([]byte(step1encoded)), step1)

	step2encoded, _ := encoder.Bytes([]byte(step2))
	for ; len(step2encoded)%16 != 0; step2encoded = append(step2encoded, byte(' ')) {
	}
	cipher, _ := aes.NewCipher([]byte("jo6aey6haid2Teih"))
	result := make([]byte, len(step2encoded))
	for bs, be := 0, 16; bs < len(step2encoded); bs, be = bs+16, be+16 {
		cipher.Encrypt(result[bs:be], step2encoded[bs:be])
	}
	return fmt.Sprintf("%x", result)
}
func (client *Client) getBlowfishKey(trackInfo pageTrackData) (bfKey []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			bfKey = nil
			err = errors.New("getBlowfishKey: Panicked")
			log.Println(r)
		}
	}()
	SECRET := "g4el58wc0zvf9na1"
	encoder := charmap.Windows1252.NewEncoder()
	sngid, _ := encoder.Bytes([]byte(trackInfo.SNGId))
	idMd5 := fmt.Sprintf("%x", md5.Sum(sngid))
	bfKey = make([]byte, 16)
	for i := range bfKey {
		bfKey[i] = idMd5[i] ^ idMd5[i+16] ^ SECRET[i]
	}
	return
}
func (client *Client) getTrackDownloadURL(trackInfo pageTrackData) (url string, err error) {
	defer func() {
		if r := recover(); r != nil {
			url = ""
			err = errors.New("getTrackDownloadURL Panicked")
			log.Println(r)
		}
	}()
	cdn := trackInfo.MD5Origin[0]
	url = strings.Join([]string{"https://e-cdns-proxy-", string(cdn), ".dzcdn.net/mobile/1/", client.getSongFileName(trackInfo)}, "")
	return
}

//PopulateMetadata populates the required metadata for downloading the track
func (client *Client) PopulateMetadata(dTrack *Track) (err error) {
	if client == nil {
		err = errors.New("PopulateMetadata: nil Deezer Client")
	}
	trackInfo, err := client.getTrackInfo(dTrack.deezerTrack.ID, false)
	if err != nil {
		return
	}
	dTrack.StreamURL, err = client.getTrackDownloadURL(trackInfo)
	if err != nil {
		return
	}
	dTrack.BlowfishKey, err = client.getBlowfishKey(trackInfo)
	if err != nil {
		return
	}
	return
}

//GetTrackByID returns the populated track with the provided ID on Deezer
func (client *Client) GetTrackByID(trackID string) (track common.Track, err error) {
	var url string
	var dTrack deezerTrack
	url = fmt.Sprintf("https://api.deezer.com/track/%s", trackID)
	response, err := http.Get(url)
	if err != nil {
		return
	}
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&dTrack)
	_, _, _, _, sURI, err := client.spotifyClient.SearchTrack("", "", "", dTrack.ISRC)
	if err == nil && len(sURI) > 0 {
		dTrack.SpotifyURI = sURI
	}

	itrack := &Track{deezerTrack: dTrack, playID: common.GenerateID()}
	err = client.PopulateMetadata(itrack)
	track = itrack
	return
}

//SearchTrack takes the track title and optional track's artist query and returns the best match track on Deezer
func (client *Client) SearchTrack(track, artist string) ([]common.Track, error) {
	var url string
	var sTrack, sArtist, sAlbum, sISRC, sURI string
	var err error
	withSpotify := client.spotifyClient != nil
	withISRC := withSpotify
start:
	if len(artist) == 0 && withSpotify {
		sTrack, sArtist, sAlbum, sISRC, sURI, err = client.spotifyClient.SearchTrackQuery(track)
		if err != nil {
			log.Printf("spotifyClient.SearchTrack() failed: %v\n", err)
			withSpotify = false
			goto start
		} else {
			if withISRC && len(sISRC) > 0 {
				url = fmt.Sprint("https://api.deezer.com/2.0/track/isrc:", sISRC)
			} else {
				url = fmt.Sprintf("https://api.deezer.com/search/track/?q=track:\"%s\"artist:\"%s\"album:\"%s\"", template.URLQueryEscaper(sTrack), template.URLQueryEscaper(sArtist), template.URLQueryEscaper(sAlbum))
			}
		}
	} else {
		if len(artist) == 0 {
			url = fmt.Sprintf("https://api.deezer.com/search/track/?q=%s", template.URLQueryEscaper(track))
		} else {
			url = fmt.Sprintf("https://api.deezer.com/search/track/?q=track:\"%s\"artist:\"%s\"", template.URLQueryEscaper(track), template.URLQueryEscaper(artist))
		}
	}
	response, err := http.Get(url)
	if err != nil {
		if withSpotify {
			if withISRC {
				log.Println("Search with spotify ISRC failed")
				withISRC = false
				goto start
			}
			log.Println("Search with spotify failed")
			withSpotify = false
			goto start
		}
		return nil, err
	}
	defer response.Body.Close()
	var resp searchTrackResponse
	if withSpotify && withISRC {
		resp = searchTrackResponse{Data: make([]deezerTrack, 1)}
		err = json.NewDecoder(response.Body).Decode(&resp.Data[0])
		if resp.Data[0].ID == 0 {
			err = errors.New("ISRC not found on deezer")
		}
	} else {
		err = json.NewDecoder(response.Body).Decode(&resp)
	}
	if err != nil {
		if withSpotify {
			if withISRC {
				log.Println("Search with spotify ISRC failed")
				withISRC = false
			} else {
				log.Println("Search with spotify failed")
				withSpotify = false
			}
			goto start
		}
		return nil, err
	}
	itracks := resp.Data
	if len(itracks) <= 0 {
		if withISRC {
			log.Println("Search with spotify ISRC failed")
			withISRC = false
			goto start
		}
		if withSpotify {
			log.Println("Search with spotify failed")
			withSpotify = false
			goto start
		}
		return nil, errors.New("No track found")
	}
	tracks := make([]common.Track, len(itracks))
	for i, v := range itracks {

		if withSpotify && (v.ISRC == sISRC || (v.Title == sTrack && v.Artist.Name == sArtist && v.Album.Title == sAlbum)) {
			v.SpotifyURI = sURI
			if withISRC && i == 0 {
				var sURI string
				_, _, _, _, sURI, err = client.spotifyClient.SearchTrack(v.Title, v.Artist.Name, v.Album.Title, v.ISRC)
				if err == nil && len(sURI) > 0 {
					v.SpotifyURI = sURI
				}
			}
		}
		v.client = client
		tracks[i] = &Track{deezerTrack: v, playID: common.GenerateID()}
	}
	return tracks, nil
}
