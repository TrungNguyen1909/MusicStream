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
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/csn"
	"github.com/TrungNguyen1909/MusicStream/deezer"
	"github.com/TrungNguyen1909/MusicStream/lyrics"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	_ "github.com/joho/godotenv/autoload"
)

func preloadTrack(stream io.ReadCloser, quit chan int) {
	var encodedTime time.Duration
	streamer, format, err := mp3.Decode(stream)
	if err != nil {
		log.Panic(err)
	}
	defer streamer.Close()
	var needResampling bool
	var resampled *beep.Resampler
	if format.SampleRate != beep.SampleRate(48000) {
		resampled = beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
		format.SampleRate = beep.SampleRate(48000)
		needResampling = true
	}
	defer endCurrentStream()
	pushSilentFrames(&encodedTime)
	defer pushSilentFrames(&encodedTime)
	pos := int64(encoder.GranulePos())
	atomic.StoreInt64(&startPos, pos)
	deltaChannel <- pos

	samples := make([][2]float64, 960)
	buf := make([]byte, len(samples)*format.Width())
	for {
		select {
		case <-quit:
			return
		default:
		}
		var (
			n  int
			ok bool
		)
		if needResampling {
			n, ok = resampled.Stream(samples)
		} else {
			n, ok = streamer.Stream(samples)
		}
		if !ok {
			break
		}
		for i, sample := range samples {
			switch {
			case format.Precision == 1:
				format.EncodeUnsigned(buf[i*format.Width():], sample)
			case format.Precision == 2 || format.Precision == 3:
				format.EncodeSigned(buf[i*format.Width():], sample)
			default:
				panic(fmt.Errorf("encode: invalid precision: %d", format.Precision))
			}
		}
		pushPCMAudio(buf[:n*format.Width()], &encodedTime)
		if 0 <= n && n < len(samples) && ok {
			return
		}
	}
}
func processTrack() {
	defer func() {
		if r := recover(); r != nil {
			watchDog++
			log.Println("Panicked!!!:", r)
			if currentTrack.Source() == common.Deezer {
				log.Println("Creating a new deezer client...")
				dzClient = deezer.NewClient()
			}
			log.Println("Resuming...")
		}
	}()
	var track common.Track
	var err error
	radioStarted := false
	if currentTrackID == -1 || watchDog >= 3 || currentTrack.Source() == common.CSN {
		if playQueue.Empty() {
			radioStarted = true
			go processRadio(quitRadio)
		}
		activityWg.Wait()
		track = playQueue.Pop().(common.Track)
		dequeueCallback()
		currentTrackID = -1
		watchDog = 0
	} else {
		dtrack := currentTrack.(deezer.Track)
		err = dzClient.PopulateMetadata(&dtrack)
		track = dtrack
		if err != nil {
			currentTrackID = -1
			watchDog = 0
			return
		}
	}
	activityWg.Wait()
	currentTrackID = track.ID()
	currentTrack = track
	if radioStarted {
		quitRadio <- 0
	}
	if track.Source() == common.CSN {
		cTrack := track.(csn.Track)
		err = cTrack.Populate()
		if err != nil {
			log.Panic(err)
		}
		track = cTrack
	}
	log.Printf("Playing %v - %v\n", track.Title(), track.Artist())
	trackDict := common.GetMetadata(track)
	var mxmlyrics common.LyricsResult
	mxmlyrics, err = lyrics.GetLyrics(track.Title(), track.Artist(), track.Album(), track.Artists(), track.SpotifyURI(), track.Duration())
	if err == nil {
		trackDict.Lyrics = mxmlyrics
	}
	stream, err := track.Download()
	if err != nil {
		log.Panic(err)
	}
	quit := make(chan int, 10)
	if radioStarted {
		<-quitRadio
	}
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
	currentTrackID = -1
	watchDog = 0
}
