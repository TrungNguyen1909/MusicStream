package main

import (
	"log"
	"os"
	"path/filepath"
	"plugin"

	"github.com/TrungNguyen1909/MusicStream"
	"github.com/TrungNguyen1909/MusicStream/server"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	var config server.Config
	if staticFilesPath, ok := os.LookupEnv("WWW"); ok && len(staticFilesPath) > 0 {
		config.StaticFilesPath = staticFilesPath
	}
	if mxmUserToken, ok := os.LookupEnv("MUSIXMATCH_USER_TOKEN"); !ok {
		log.Println("Warning: Musixmatch token not found")
	} else {
		config.MusixMatchUserToken = mxmUserToken
		if mxmOBUserToken, ok := os.LookupEnv("MUSIXMATCH_OB_USER_TOKEN"); ok {
			config.MusixMatchOBUserToken = mxmOBUserToken
		}
	}
	pluginsPath, err := filepath.Glob("plugins/**/*.plugin")
	if err != nil {
		log.Panic("Cannot find any plugins")
	}
	for _, path := range pluginsPath {
		p, err := plugin.Open(path)
		if err != nil {
			log.Println("plugin.Open: ", err)
		}
		config.Plugins = append(config.Plugins, p)
	}
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}
	port = ":" + port
	log.Printf("Intializing MusicStream v%s...", MusicStream.Version)
	s := server.NewServer(config)
	defer s.Close()
	log.Panic(s.Start(port))
}
