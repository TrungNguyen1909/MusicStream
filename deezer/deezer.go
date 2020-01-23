package deezer

import (
	"bytes"
	"common"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/json"
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
	"strconv"
	"strings"

	"golang.org/x/crypto/blowfish"
	"golang.org/x/text/encoding/charmap"
)

const (
	deezerURL        = "https://www.deezer.com"
	ajaxActionURL    = "https://www.deezer.com/ajax/action.php"
	unofficialAPIURL = "https://www.deezer.com/ajax/gw-light.php"
)

var arlCookie = "***REMOVED***"

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
	Data []common.Track `json:"data"`
}

type byRank []common.Track

func (p byRank) Len() int {
	return len(p)
}

func (p byRank) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p byRank) Less(i, j int) bool {
	return p[i].Rank > p[j].Rank
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
		log.Fatal(err)
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
	client.arlCookie = &http.Cookie{Name: "arl", Value: arlCookie}
	client.deezerURL, _ = url.Parse(deezerURL)
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
			panic("Failed to get trackInfo adequately")
		}
		client.initDeezerAPI()
		return client.getTrackInfo(trackID, true)
	}
	return resp.Results.Data, nil
}
func (client *Client) getSongFileName(trackInfo pageTrackData, trackQualityID int) string {
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
func (client *Client) getBlowfishKey(trackInfo pageTrackData) []byte {
	SECRET := "g4el58wc0zvf9na1"
	encoder := charmap.Windows1252.NewEncoder()
	sngid, _ := encoder.Bytes([]byte(trackInfo.SNGId))
	idMd5 := fmt.Sprintf("%x", md5.Sum(sngid))
	bfKey := make([]byte, 16)
	for i := range bfKey {
		bfKey[i] = idMd5[i] ^ idMd5[i+16] ^ SECRET[i]
	}
	return bfKey
}
func (client *Client) getTrackDownloadURL(trackInfo pageTrackData, trackQualityID int) string {
	cdn := trackInfo.MD5Origin[0]
	return strings.Join([]string{"https://e-cdns-proxy-", string(cdn), ".dzcdn.net/mobile/1/", client.getSongFileName(trackInfo, trackQualityID)}, "")
}

func (client *Client) downloadTrack(trackInfo pageTrackData, trackQualityID int) (io.ReadCloser, error) {
	trackurl := client.getTrackDownloadURL(trackInfo, trackQualityID)
	// fmt.Printf("%x\n", trackurl)
	response, err := client.httpClient.Get(trackurl)
	if err != nil {
		return nil, err
	}
	return &trackDecrypter{r: response.Body, BlowfishKey: client.getBlowfishKey(trackInfo)}, nil
}

func (client *Client) GetTrackByID(trackID int) (track common.Track, err error) {
	var url string
	url = fmt.Sprintf("https://api.deezer.com/track/%d", trackID)
	response, err := http.Get(url)
	if err != nil {
		return
	}
	err = json.NewDecoder(response.Body).Decode(&track)
	return
}

func (client *Client) SearchTrack(track, artist string) ([]common.Track, error) {
	var url string
	if len(artist) == 0 {
		url = fmt.Sprintf("https://api.deezer.com/search?q=%s", template.URLQueryEscaper(track))
	} else {
		url = fmt.Sprintf("https://api.deezer.com/search?q=track:\"%s\"artist:\"%s\"", template.URLQueryEscaper(track), template.URLQueryEscaper(artist))
	}
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	var resp searchTrackResponse
	body, _ := ioutil.ReadAll(response.Body)
	err = json.Unmarshal(body, &resp)

	if err != nil {
		return nil, err
	}
	tracks := resp.Data
	// if !strings.Contains(track, ":") {
	// 	sort.Sort(byRank(tracks))
	// }
	return tracks, nil
}

func (client *Client) DownloadTrack(trackID int, trackQualityID int) (io.ReadCloser, error) {
	trackInfo, err := client.getTrackInfo(trackID, false)
	if err != nil {
		return nil, err
	}
	return client.downloadTrack(trackInfo, trackQualityID)
}

const trackQualityID = 3
