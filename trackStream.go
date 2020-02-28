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

package main

import (
	"io"
	"log"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/lyrics"
	"github.com/TrungNguyen1909/MusicStream/youtube"
	_ "github.com/joho/godotenv/autoload"
)

func preloadTrack(stream io.ReadCloser, quit chan int) {
	var encodedTime time.Duration
	defer stream.Close()
	defer endCurrentStream()
	pushSilentFrames(&encodedTime)
	defer pushSilentFrames(&encodedTime)
	pos := int64(encoder.GranulePos())
	atomic.StoreInt64(&startPos, pos)
	deltaChannel <- pos
	for {
		select {
		case <-quit:
			return
		default:
		}
		buf := make([]byte, 3840)
		n, err := stream.Read(buf)
		pushPCMAudio(buf[:n], &encodedTime)
		if err != nil {
			return
		}
	}
}
func processTrack() {
	defer func() {
		if r := recover(); r != nil {
			watchDog++
			log.Println("processTrack Panicked:", r)
		}
	}()
	var track common.Track
	var err error
	radioStarted := false
	if playQueue.Empty() {
		radioStarted = true
		go processRadio(quitRadio)
	}
	activityWg.Wait()
	track = playQueue.Pop().(common.Track)
	dequeueCallback()
	currentTrackID = ""
	watchDog = 0
	activityWg.Wait()
	currentTrackID = track.ID()
	currentTrack = track
	if radioStarted {
		quitRadio <- 0
	}
	log.Printf("Playing %v - %v\n", track.Title(), track.Artist())
	trackDict := common.GetMetadata(track)
	var mxmlyrics common.LyricsResult
	if track.Source() != common.Youtube {
		mxmlyrics, err = lyrics.GetLyrics(track.Title(), track.Artist(), track.Album(), track.Artists(), track.ISRC(), track.SpotifyURI(), track.Duration())
		if err == nil {
			trackDict.Lyrics = mxmlyrics
		}
	} else {
		ytsub, err := youtube.GetLyrics(track.ID())
		if err == nil {
			trackDict.Lyrics = ytsub
		}
	}
	stream, err := track.Download()
	if err != nil {
		log.Panic(err)
	}
	quit := make(chan int, 10)
	go preloadTrack(stream, quit)
	for len(skipChannel) > 0 {
		select {
		case <-skipChannel:
		default:
		}
	}
	time.Sleep(time.Until(etaDone.Load().(time.Time)))
	startTime = time.Now()
	setTrack(trackDict)
	streamToClients(skipChannel, quit)
	log.Println("Stream ended!")
	currentTrackID = ""
	watchDog = 0
}
