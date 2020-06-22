package main

import (
	"log"
	"os"

	"github.com/TrungNguyen1909/MusicStream"
	"github.com/TrungNguyen1909/MusicStream/server"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	var config server.Config
	if deezerARL, ok := os.LookupEnv("DEEZER_ARL"); !ok {
		log.Println("Warning: Deezer token not found")
	} else {
		config.DeezerARL = deezerARL
	}
	if spotifyClientID, ok := os.LookupEnv("SPOTIFY_CLIENT_ID"); !ok {
		log.Println("Warning: no spotify token found")
	} else {
		if spotifyClientSecret, ok := os.LookupEnv("SPOTIFY_CLIENT_SECRET"); !ok {
			log.Println("Warning: no spotify token found")
		} else {
			config.SpotifyClientID = spotifyClientID
			config.SpotifyClientSecret = spotifyClientSecret
		}
	}
	if mxmUserToken, ok := os.LookupEnv("MUSIXMATCH_USER_TOKEN"); !ok {
		log.Println("Warning: Musixmatch token not found")
	} else {
		config.MusixMatchUserToken = mxmUserToken
		if mxmOBUserToken, ok := os.LookupEnv("MUSIXMATCH_OB_USER_TOKEN"); ok {
			config.MusixMatchOBUserToken = mxmOBUserToken
		}
	}

	if ytDevKey, ok := os.LookupEnv("YOUTUBE_DEVELOPER_KEY"); !ok {
		log.Println("Warning: Youtube Data API v3 key not found")
	} else {
		config.YoutubeDeveloperKey = ytDevKey
	}
	if radioEnabled, ok := os.LookupEnv("RADIO_ENABLED"); ok && radioEnabled == "1" {
		config.RadioEnabled = true
	}
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}
	port = ":" + port
	log.Printf("Intializing MusicStream v%s...", MusicStream.Version)
	s := server.NewServer(config)
	log.Fatal(s.Start(port))
}
