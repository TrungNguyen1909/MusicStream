package lyrics

import (
	"common"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
)

type mxmResponse struct {
	Message struct {
		Body struct {
			MacroCalls struct {
				MatcherTrackGet struct {
					Message struct {
						Body struct {
							Track struct {
								AlbumCoverart100x100  string     `json:"album_coverart_100x100"`
								AlbumCoverart350x350  string     `json:"album_coverart_350x350"`
								AlbumCoverart500x500  string     `json:"album_coverart_500x500"`
								AlbumCoverart800x800  string     `json:"album_coverart_800x800"`
								AlbumID               int        `json:"album_id"`
								AlbumName             string     `json:"album_name"`
								ArtistID              int        `json:"artist_id"`
								ArtistMbid            string     `json:"artist_mbid"`
								ArtistName            string     `json:"artist_name"`
								CommontrackID         int        `json:"commontrack_id"`
								CommontrackIsrcs      [][]string `json:"commontrack_isrcs"`
								CommontrackSpotifyIds []string   `json:"commontrack_spotify_ids"`
								CommontrackVanityID   string     `json:"commontrack_vanity_id"`
								Explicit              int        `json:"explicit"`
								FirstReleaseDate      string     `json:"first_release_date"`
								HasLyrics             int        `json:"has_lyrics"`
								HasLyricsCrowd        int        `json:"has_lyrics_crowd"`
								HasRichsync           int        `json:"has_richsync"`
								HasSubtitles          int        `json:"has_subtitles"`
								Instrumental          int        `json:"instrumental"`
								LyricsID              int        `json:"lyrics_id"`
								NumFavourite          int        `json:"num_favourite"`
								PrimaryGenres         struct {
									MusicGenreList []struct {
										MusicGenre struct {
											MusicGenreID           int    `json:"music_genre_id"`
											MusicGenreName         string `json:"music_genre_name"`
											MusicGenreNameExtended string `json:"music_genre_name_extended"`
											MusicGenreParentID     int    `json:"music_genre_parent_id"`
											MusicGenreVanity       string `json:"music_genre_vanity"`
										} `json:"music_genre"`
									} `json:"music_genre_list"`
								} `json:"primary_genres"`
								Restricted      int `json:"restricted"`
								SecondaryGenres struct {
									MusicGenreList []struct {
										MusicGenre struct {
											MusicGenreID           int    `json:"music_genre_id"`
											MusicGenreName         string `json:"music_genre_name"`
											MusicGenreNameExtended string `json:"music_genre_name_extended"`
											MusicGenreParentID     int    `json:"music_genre_parent_id"`
											MusicGenreVanity       string `json:"music_genre_vanity"`
										} `json:"music_genre"`
									} `json:"music_genre_list"`
								} `json:"secondary_genres"`
								SubtitleID               int           `json:"subtitle_id"`
								TrackEditURL             string        `json:"track_edit_url"`
								TrackID                  int           `json:"track_id"`
								TrackIsrc                string        `json:"track_isrc"`
								TrackLength              int           `json:"track_length"`
								TrackMbid                string        `json:"track_mbid"`
								TrackName                string        `json:"track_name"`
								TrackNameTranslationList []interface{} `json:"track_name_translation_list"`
								TrackRating              int           `json:"track_rating"`
								TrackShareURL            string        `json:"track_share_url"`
								TrackSoundcloudID        int           `json:"track_soundcloud_id"`
								TrackSpotifyID           string        `json:"track_spotify_id"`
								TrackXboxmusicID         string        `json:"track_xboxmusic_id"`
								UpdatedTime              string        `json:"updated_time"`
							} `json:"track"`
						} `json:"body"`
						Header struct {
							Cached      int     `json:"cached"`
							Confidence  int     `json:"confidence"`
							ExecuteTime float64 `json:"execute_time"`
							Mode        string  `json:"mode"`
							StatusCode  int     `json:"status_code"`
						} `json:"header"`
					} `json:"message"`
				} `json:"matcher.track.get"`
				TrackLyricsGet struct {
					Message struct {
						Body struct {
							CrowdLyricsList []interface{} `json:"crowd_lyrics_list"`
							Lyrics          struct {
								ActionRequested           string `json:"action_requested"`
								BacklinkURL               string `json:"backlink_url"`
								CanEdit                   int    `json:"can_edit"`
								Explicit                  int    `json:"explicit"`
								HTMLTrackingURL           string `json:"html_tracking_url"`
								Instrumental              int    `json:"instrumental"`
								Locked                    int    `json:"locked"`
								LyricsBody                string `json:"lyrics_body"`
								LyricsCopyright           string `json:"lyrics_copyright"`
								LyricsID                  int    `json:"lyrics_id"`
								LyricsLanguage            string `json:"lyrics_language"`
								LyricsLanguageDescription string `json:"lyrics_language_description"`
								LyricsTranslated          struct {
									HTMLTrackingURL   string `json:"html_tracking_url"`
									LyricsBody        string `json:"lyrics_body"`
									PixelTrackingURL  string `json:"pixel_tracking_url"`
									Restricted        int    `json:"restricted"`
									ScriptTrackingURL string `json:"script_tracking_url"`
									SelectedLanguage  string `json:"selected_language"`
								} `json:"lyrics_translated"`
								PixelTrackingURL  string        `json:"pixel_tracking_url"`
								PublishedStatus   int           `json:"published_status"`
								PublisherList     []interface{} `json:"publisher_list"`
								Restricted        int           `json:"restricted"`
								ScriptTrackingURL string        `json:"script_tracking_url"`
								UpdatedTime       string        `json:"updated_time"`
								Verified          int           `json:"verified"`
								WriterList        []interface{} `json:"writer_list"`
							} `json:"lyrics"`
						} `json:"body"`
						Header struct {
							ExecuteTime float64 `json:"execute_time"`
							StatusCode  int     `json:"status_code"`
						} `json:"header"`
					} `json:"message"`
				} `json:"track.lyrics.get"`
				TrackSnippetGet struct {
					Message struct {
						Body struct {
							Snippet struct {
								HTMLTrackingURL   string `json:"html_tracking_url"`
								Instrumental      int    `json:"instrumental"`
								PixelTrackingURL  string `json:"pixel_tracking_url"`
								Restricted        int    `json:"restricted"`
								ScriptTrackingURL string `json:"script_tracking_url"`
								SnippetBody       string `json:"snippet_body"`
								SnippetID         int    `json:"snippet_id"`
								SnippetLanguage   string `json:"snippet_language"`
								UpdatedTime       string `json:"updated_time"`
							} `json:"snippet"`
						} `json:"body"`
						Header struct {
							ExecuteTime float64 `json:"execute_time"`
							StatusCode  int     `json:"status_code"`
						} `json:"header"`
					} `json:"message"`
				} `json:"track.snippet.get"`
				TrackSubtitlesGet struct {
					Message struct {
						Body struct {
							SubtitleList []struct {
								Subtitle struct {
									HTMLTrackingURL             string        `json:"html_tracking_url"`
									LyricsCopyright             string        `json:"lyrics_copyright"`
									PixelTrackingURL            string        `json:"pixel_tracking_url"`
									PublisherList               []interface{} `json:"publisher_list"`
									Restricted                  int           `json:"restricted"`
									ScriptTrackingURL           string        `json:"script_tracking_url"`
									SubtitleAvgCount            int           `json:"subtitle_avg_count"`
									SubtitleBody                string        `json:"subtitle_body"`
									SubtitleID                  int           `json:"subtitle_id"`
									SubtitleLanguage            string        `json:"subtitle_language"`
									SubtitleLanguageDescription string        `json:"subtitle_language_description"`
									SubtitleLength              int           `json:"subtitle_length"`
									SubtitleTranslated          struct {
										HTMLTrackingURL   string `json:"html_tracking_url"`
										PixelTrackingURL  string `json:"pixel_tracking_url"`
										ScriptTrackingURL string `json:"script_tracking_url"`
										SelectedLanguage  string `json:"selected_language"`
										SubtitleBody      string `json:"subtitle_body"`
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
				UserblobGet struct {
					Message struct {
						Header struct {
							StatusCode int `json:"status_code"`
						} `json:"header"`
					} `json:"message"`
					Meta struct {
						LastUpdated string `json:"last_updated"`
						StatusCode  int    `json:"status_code"`
					} `json:"meta"`
				} `json:"userblob.get"`
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

//GetLyrics returns the lyrics of the song with provided information
func GetLyrics(track, artist, album, artists, SpotifyURI string, duration int) (result common.LyricsResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("GetLyrics: %v\n", r)
		}
	}()
	cookiesJar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: cookiesJar}
	rawURL := "http://apic.musixmatch.com/ws/1.1/macro.subtitles.get?format=json&user_language=en&tags=playing&namespace=lyrics_synched&f_subtitle_length_max_deviation=1&subtitle_format=mxm&app_id=mac-ios-v2.0&part=subtitle_translated%2Clyrics_translated&selected_language=en&usertoken=" + os.Getenv("MUSIXMATCH_USER_TOKEN")
	reqURL, _ := url.Parse(rawURL)
	queries := reqURL.Query()
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
	}
	queries.Add("f_subtitle_length", strconv.Itoa(duration))
	if len(SpotifyURI) > 0 {
		queries.Add("track_spotify_id", SpotifyURI)
	}
	log.Printf("Spotify URI: %s\n", SpotifyURI)
	reqURL.RawQuery = queries.Encode()
	req, _ := http.NewRequest("GET", reqURL.String(), nil)
	req.Header.Set("Host", "apic.musixmatch.com")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_2) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.4 Safari/605.1.15")
	req.Header.Set("Accept-Language", "en-us")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
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
		log.Println(err)
		return
	}
	result = common.LyricsResult{}
	result.RawLyrics = d.Message.Body.MacroCalls.TrackLyricsGet.Message.Body.Lyrics.LyricsBody
	subtitle := d.Message.Body.MacroCalls.TrackSubtitlesGet.Message.Body.SubtitleList[0].Subtitle
	result.Language = subtitle.SubtitleLanguage
	sd := subtitle.SubtitleBody
	var syncedLyrics []common.LyricsLine
	if result.Language != "en" {
		st := subtitle.SubtitleTranslated.SubtitleBody
		var subtitleTranslated []common.LyricsLine
		err = json.Unmarshal(([]byte)(st), &subtitleTranslated)
		if err != nil {
			log.Println(err)
			return
		}
		syncedLyrics = make([]common.LyricsLine, len(subtitleTranslated))
		for i, v := range subtitleTranslated {
			syncedLyrics[i].Translated = v.Text
			syncedLyrics[i].Text = v.Original
			syncedLyrics[i].Time = v.Time
		}
		result.SyncedLyrics = syncedLyrics
	} else {
		err = json.Unmarshal(([]byte)(sd), &syncedLyrics)
		if err != nil {
			log.Println(err)
			return
		}
		result.SyncedLyrics = syncedLyrics
	}
	return
}
