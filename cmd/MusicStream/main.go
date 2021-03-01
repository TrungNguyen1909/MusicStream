package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"plugin"
	"syscall"
	"time"

	"github.com/TrungNguyen1909/MusicStream"
	"github.com/TrungNguyen1909/MusicStream/server"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	var config server.Config
	if staticFilesPath, ok := os.LookupEnv("WWW"); ok && len(staticFilesPath) > 0 {
		config.StaticFilesPath = staticFilesPath
	}
	if defaultSource, ok := os.LookupEnv("DEFAULT_SOURCE"); ok && len(defaultSource) > 0 {
		config.DefaultMusicSource = defaultSource
	}
	if mxmUserToken, ok := os.LookupEnv("MUSIXMATCH_USER_TOKEN"); !ok {
		log.Println("[main] Warning: Musixmatch token not found")
	} else {
		config.MusixMatchUserToken = mxmUserToken
		if mxmOBUserToken, ok := os.LookupEnv("MUSIXMATCH_OB_USER_TOKEN"); ok {
			config.MusixMatchOBUserToken = mxmOBUserToken
		}
	}
	log.Printf("[main] Intializing MusicStream v%s...", MusicStream.Version)
	pluginsPath, err := filepath.Glob("plugins/**/*.plugin")
	if err != nil {
		log.Panic("[main] Cannot find any plugins")
	}
	for _, path := range pluginsPath {
		p, err := plugin.Open(path)
		if err != nil {
			log.Println("[main] plugin.Open: ", err)
		} else {
			config.Plugins = append(config.Plugins, p)
		}
	}
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}
	port = ":" + port
	s := server.NewServer(config)
	defer s.Close()
	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		signal.Notify(sigint, syscall.SIGTERM)
		<-sigint
		log.Println("[main] Recevied interrupt, shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = s.Shutdown(ctx); err != nil {
			log.Printf("[main] Server shutdown: %+v", err)
		}
		close(idleConnsClosed)
	}()
	if err = s.Start(port); err != http.ErrServerClosed {
		log.Printf("[main] Server stopped: %+v", err)
	}
	<-idleConnsClosed
}
