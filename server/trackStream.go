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
	"context"
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
)

func (s *Server) preloadTrack(stream io.ReadCloser, streamContext context.Context) {
	s.streamMux.Lock()
	defer s.streamMux.Unlock()
	defer stream.Close()
	defer s.endCurrentStream()
	s.pushSilentFrames()
	defer s.pushSilentFrames()
	pos := int64(s.vorbisEncoder.GranulePos())
	atomic.StoreInt64(&s.startPos, pos)
	s.deltaChannel <- pos
	log.Println("[MusicStream] Track preloading started")
	defer log.Println("[MusicStream] Track preloading done")
	for {
		select {
		case <-streamContext.Done():
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
			log.Printf("[MusicStream] processTrack ERROR: %+v", r)
		}
	}()
	var track common.Track
	var err error
	if s.playQueue.Empty() && s.radioTrack != nil {
		radioStreamContext, cancelRadio := context.WithCancel(context.TODO())
		if s.cancelRadio != nil {
			s.cancelRadio()
		}
		go s.processRadio(radioStreamContext)
		s.cancelRadio = cancelRadio
	} else if s.playQueue.Empty() {
		s.currentTrack = s.defaultTrack
		pos := int64(s.vorbisEncoder.GranulePos())
		atomic.StoreInt64(&s.startPos, pos)
		s.deltaChannel <- pos
		s.setTrack(common.GetMetadata(s.currentTrack))
	}
	s.activityWg.Wait()
	track = s.playQueue.Pop().(common.Track)
	s.activityWg.Wait()
	s.currentTrack = track
	if s.cancelRadio != nil {
		s.cancelRadio()
	}
	log.Printf("[MusicStream] Playing %v - %v\n", track.Title(), track.Artist())
	trackDict := common.GetMetadata(track)
	if ltrack, ok := track.(common.TrackWithLyrics); ok {
		lyrics, err := ltrack.GetLyrics()
		if err != nil {
			log.Println("[MusicStream] track.GetLyrics: ERROR: ", err)
		} else {
			trackDict.Lyrics = lyrics
		}
	} else if s.mxmClient != nil {
		lyrics, err := s.mxmClient.GetLyrics(track)
		if err != nil {
			log.Println("[MusixMatch] GetLyrics: ", err)
		} else {
			trackDict.Lyrics = lyrics
		}
	}
	stream, err := track.Stream()
	if err != nil {
		log.Panicf("[MusicStream] track.Stream: ERROR: %+v", err)
	}
	rawStream, err := GetRawStream(stream)
	if err != nil {
		log.Panicf("[MusicStream] GetRawStream: ERROR: %+v", err)
	}
	streamContext, skipFunc := context.WithCancel(context.TODO())
	go s.preloadTrack(rawStream, streamContext)
	time.Sleep(time.Until(s.lastStreamEnded))
	s.startTime = time.Now()
	s.setTrack(trackDict)
	s.streamContext = streamContext
	s.skipFunc = skipFunc
	s.lastStreamEnded = s.streamToClients(streamContext)
	s.skipFunc()
}
