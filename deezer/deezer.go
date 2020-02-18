package deezer

import (
	"bytes"
	"common"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"spotify"
	"strconv"
	"strings"

	"golang.org/x/crypto/blowfish"
	"golang.org/x/text/encoding/charmap"
)

const (
	deezerURL        = "https://www.deezer.com"
	ajaxActionURL    = "https://www.deezer.com/ajax/action.php"
	unofficialAPIURL = "https://www.deezer.com/ajax/gw-light.php"
	trackQualityID   = 3
)

type Artist struct {
	Name string `json:"name"`
}
type Album struct {
	Title       string `json:"title"`
	Cover       string `json:"cover"`
	CoverSmall  string `json:"cover_small"`
	CoverMedium string `json:"cover_medium"`
	CoverBig    string `json:"cover_big"`
	CoverXL     string `json:"cover_xl"`
}
type Track struct {
	deezerTrack
	StreamURL   string
	BlowfishKey []byte
}

func (track Track) ID() int {
	return track.deezerTrack.ID
}

func (track Track) Title() string {
	return track.deezerTrack.Title
}

func (track Track) Album() string {
	return track.deezerTrack.Album.Title
}

func (track Track) Source() int {
	return common.Deezer
}

func (track Track) Artist() string {
	return track.deezerTrack.Artist.Name
}
func (track Track) Artists() string {
	artists := ""
	for _, v := range track.deezerTrack.Contributors {
		artists = strings.Join([]string{artists, v.Name}, ", ")
	}
	artists = artists[2:]
	return artists
}
func (track Track) Duration() int {
	return track.deezerTrack.Duration
}

func (track Track) CoverURL() string {
	return track.deezerTrack.Album.CoverXL
}

func (track Track) Download() (io.ReadCloser, error) {
	if track.StreamURL == "" || len(track.BlowfishKey) == 0 {
		return nil, errors.New("Metadata not yet populated")
	}
	response, err := http.Get(track.StreamURL)
	if err != nil {
		return nil, err
	}
	return &trackDecrypter{r: response.Body, BlowfishKey: track.BlowfishKey}, nil
}
func (track Track) SpotifyURL() string {
	return track.deezerTrack.SpotifyURL
}

func (track *Track) SetSpotifyURL(sURI string) {
	track.deezerTrack.SpotifyURL = sURI
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
	SpotifyURL   string
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
	client.ajaxActionURL, _ = url.Parse(ajaxActionURL)
	client.unofficialAPIURL, _ = url.Parse(unofficialAPIURL)
	client.unofficialAPIQuery.Set("api_version", "1.0")
	client.unofficialAPIQuery.Set("input", "3")
	client.unofficialAPIQuery.Set("api_token", "")
	client.arlCookie = &http.Cookie{Name: "arl", Value: os.Getenv("DEEZER_ARL")}
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
func (client *Client) makeRequest(url string, body []byte) *http.Request {
	request, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	request.Header = client.httpHeaders
	request.AddCookie(client.arlCookie)
	return request
}
func (client *Client) makeUnofficialAPIRequest(method string, body []byte) *http.Request {

	client.unofficialAPIQuery.Set("method", method)
	client.unofficialAPIQuery.Set("cid", getAPICID())
	client.unofficialAPIURL.RawQuery = client.unofficialAPIQuery.Encode()
	return client.makeRequest(client.unofficialAPIURL.String(), body)
}
func (client *Client) initDeezerAPI() {
	request := client.makeUnofficialAPIRequest("deezer.getUserData", []byte(""))
	response, err := client.httpClient.Do(request)
	if err != nil {
		log.Println(err)
		return
	}
	buf, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	var resp getUserDataResponse
	json.Unmarshal(buf, &resp)
	if len(resp.Results.CheckForm) <= 0 {
		log.Printf("%s\n", buf)
		return
	}
	client.unofficialAPIQuery.Set("api_token", resp.Results.CheckForm)
	log.Printf("Successfully initiated Deezer API. Checkform: \"%s\"\n", resp.Results.CheckForm)
}
func (client *Client) getTrackInfo(trackID int, secondTry bool) (pageTrackData, error) {
	data := map[string]interface{}{
		"SNG_ID": trackID,
	}
	encoded, _ := json.Marshal(data)
	request := client.makeUnofficialAPIRequest("deezer.pageTrack", encoded)
	response, _ := client.httpClient.Do(request)

	var resp pageTrackResponse
	json.NewDecoder(response.Body).Decode(&resp)
	if len(resp.Results.Data.MD5Origin) <= 0 {
		if secondTry {
			log.Panic("Failed to get trackInfo adequately")
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

func (client *Client) PopulateMetadata(dTrack *Track) (err error) {
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

func (client *Client) GetTrackByID(trackID int) (track common.Track, err error) {
	var url string
	var dTrack deezerTrack
	url = fmt.Sprintf("https://api.deezer.com/track/%d", trackID)
	response, err := http.Get(url)
	if err != nil {
		return
	}
	err = json.NewDecoder(response.Body).Decode(&dTrack)
	itrack := Track{deezerTrack: dTrack}
	err = client.PopulateMetadata(&itrack)
	track = itrack
	return
}

func (client *Client) SearchTrack(track, artist string) ([]common.Track, error) {
	var url string
	var sTrack, sArtist, sAlbum, sURI string
	var err error
	withSpotify := client.spotifyClient != nil
start:
	if len(artist) == 0 && withSpotify {
		sTrack, sArtist, sAlbum, sURI, err = client.spotifyClient.SearchTrack(track)
		if err != nil {
			log.Printf("spotifyClient.SearchTrack() failed: %v\n", err)

		} else {
			url = fmt.Sprintf("https://api.deezer.com/search/track/?q=track:\"%s\"artist:\"%s\"album:\"%s\"", template.URLQueryEscaper(sTrack), template.URLQueryEscaper(sArtist), template.URLQueryEscaper(sAlbum))
		}
	} else {
		if len(artist) == 0 {
			url = fmt.Sprintf("https://api.deezer.com/search/track/?q=\"%s\"", template.URLQueryEscaper(track))
		} else {
			url = fmt.Sprintf("https://api.deezer.com/search/track/?q=track:\"%s\"artist:\"%s\"", template.URLQueryEscaper(track), template.URLQueryEscaper(artist))
		}
	}
	response, err := http.Get(url)
	if err != nil {
		if withSpotify {
			log.Println("Search with spotify failed")
			withSpotify = false
			goto start
		}
		return nil, err
	}
	var resp searchTrackResponse
	body, _ := ioutil.ReadAll(response.Body)
	err = json.Unmarshal(body, &resp)

	if err != nil {
		if withSpotify {
			log.Println("Search with spotify failed")
			withSpotify = false
			goto start
		}
		return nil, err
	}
	itracks := resp.Data
	if len(itracks) <= 0 {
		if withSpotify {
			withSpotify = false
			goto start
		}
		return nil, errors.New("No track found")
	}
	tracks := make([]common.Track, len(itracks))
	for i, v := range itracks {

		if withSpotify && v.Title == sTrack && v.Artist.Name == sArtist && v.Album.Title == sAlbum {
			v.SpotifyURL = sURI
		}
		tracks[i] = Track{deezerTrack: v}
	}
	return tracks, nil
}
