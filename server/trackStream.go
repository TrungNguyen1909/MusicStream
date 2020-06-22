/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020 Nguyễn Hoàng Trung(TrungNguyen1909)
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

package server

import (
	"io"
	"log"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
)

func (s *Server) preloadTrack(stream io.ReadCloser, quit chan int) {
	s.streamMux.Lock()
	defer s.streamMux.Unlock()
	defer stream.Close()
	defer s.endCurrentStream()
	s.pushSilentFrames()
	defer s.pushSilentFrames()
	pos := int64(s.vorbisEncoder.GranulePos())
	atomic.StoreInt64(&s.startPos, pos)
	s.deltaChannel <- pos
	log.Println("Track preloading started")
	defer log.Println("Track preloading done")
	for {
		select {
		case <-quit:
			return
		default:
		}
		buf := make([]byte, 3840)
		n, err := stream.Read(buf)
		s.pushPCMAudio(buf[:n])
		if err != nil {
			return
		}
	}
}
func (s *Server) processTrack() {
	defer func() {
		if r := recover(); r != nil {
			s.watchDog++
			log.Println("processTrack Panicked:", r)
		}
	}()
	var track common.Track
	var err error
	radioStarted := false
	if s.playQueue.Empty() && s.radioTrack != nil {
		radioStarted = true
		go s.processRadio(s.quitRadio)
	} else if s.playQueue.Empty() {
		s.currentTrack = s.defaultTrack
		pos := int64(s.vorbisEncoder.GranulePos())
		atomic.StoreInt64(&s.startPos, pos)
		s.deltaChannel <- pos
		s.setTrack(common.GetMetadata(s.currentTrack))
	}
	s.activityWg.Wait()
	track = s.playQueue.Pop().(common.Track)
	s.currentTrackID = ""
	s.watchDog = 0
	s.activityWg.Wait()
	s.currentTrackID = track.ID()
	s.currentTrack = track
	if radioStarted {
		s.quitRadio <- 0
	}
	log.Printf("Playing %v - %v\n", track.Title(), track.Artist())
	trackDict := common.GetMetadata(track)
	var mxmlyrics common.LyricsResult
	if track.Source() != common.Youtube && s.mxmClient != nil {
		mxmlyrics, err = s.mxmClient.GetLyrics(track.Title(), track.Artist(), track.Album(), track.Artists(), track.ISRC(), track.SpotifyURI(), track.Duration())
		if err == nil {
			trackDict.Lyrics = mxmlyrics
		}
	} else if track.Source() == common.Youtube {
		ytsub, err := s.ytClient.GetLyrics(track.ID())
		if err == nil {
			trackDict.Lyrics = ytsub
		}
	}
	stream, err := track.Download()
	if err != nil {
		log.Panic("track.Download:", err)
	}
	quit := make(chan int, 10)
	go s.preloadTrack(stream, quit)
	for len(s.skipChannel) > 0 {
		<-s.skipChannel
	}
	time.Sleep(time.Until(s.lastStreamEnded))
	s.startTime = time.Now()
	s.setTrack(trackDict)
	s.lastStreamEnded = s.streamToClients(s.skipChannel, quit)
	s.currentTrackID = ""
	s.watchDog = 0
}
