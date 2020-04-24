package main

import (
	"log"
	"os"

	"github.com/TrungNguyen1909/MusicStream/server"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	if _, ok := os.LookupEnv("DEEZER_ARL"); !ok {
		log.Panic("Deezer token not found")
	}
	if _, ok := os.LookupEnv("MUSIXMATCH_USER_TOKEN"); !ok {
		log.Panic("Musixmatch token not found")
	}

	if _, ok := os.LookupEnv("YOUTUBE_DEVELOPER_KEY"); !ok {
		log.Panic("Youtube Data API v3 key not found")
	}
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}
	port = ":" + port
	s := server.NewServer()
	log.Fatal(s.Serve(port))
}
