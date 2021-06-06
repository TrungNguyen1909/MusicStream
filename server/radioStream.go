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
	"io"
	"log"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/radio"
)

func (s *Server) encodeRadio(stream io.ReadCloser, streamContext context.Context) (ended bool) {
	s.streamMux.Lock()
	defer s.streamMux.Unlock()
	defer stream.Close()
	for {
		select {
		case <-streamContext.Done():
			return true
		default:
		}
		buf := make([]byte, 3840)
		n, err := stream.Read(buf)
		s.pushPCMAudio(buf[:n])
		if err != nil {
			return false
		}
	}
}
func (s *Server) preloadRadio(streamContext context.Context) {
	log.Println("[Radio] Radio preloading started!")
	defer s.endCurrentStream()
	defer s.pushSilentFrames()
	defer log.Println("[Radio] Radio preloading stopped!")
	go func() {
		firstTime := true
		log.Println("[Radio] Starting Radio track update")
		defer log.Println("[Radio] Stopped Radio track update")
		for {
			select {
			case <-streamContext.Done():
				return
			default:
			}
			var metadata common.TrackMetadata
			c := s.radioTrack.WaitForTrackUpdate()
			if firstTime {
				firstTime = false
				metadata = common.GetMetadata(s.radioTrack)
			} else {
				select {
				case <-streamContext.Done():
					return
				case metadata = <-c:
				}
			}
			if _, ok := s.currentTrack.(*radio.Track); ok {
				s.updateStartPos(true)
				s.setTrack(metadata)
			}
		}
	}()
	for {
		stream, err := s.radioTrack.Stream()
		if err != nil {
			continue
		}
		rawStream, err := GetRawStream(stream)
		if err != nil {
			continue
		}
		if s.encodeRadio(rawStream, streamContext) {
			break
		}
	}
}
func (s *Server) processRadio(streamContext context.Context) {
	time.Sleep(time.Until(s.lastStreamEnded))
	s.radioTrack.InitWS()
	s.currentTrack = s.radioTrack
	go s.preloadRadio(streamContext)
	defer s.radioTrack.CloseWS()
	defer func() { log.Println("[MusicStream] Resuming track streaming...") }()
	s.lastStreamEnded = s.streamToClients(streamContext)
}
