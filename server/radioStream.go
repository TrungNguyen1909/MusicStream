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

func (s *Server) encodeRadio(stream io.ReadCloser, quit chan int) (ended bool) {

	defer stream.Close()
	for {
		select {
		case <-quit:
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
func (s *Server) preloadRadio(quit chan int) {
	log.Println("Radio preloading started!")
	defer s.endCurrentStream()
	defer s.pushSilentFrames()
	defer log.Println("Radio preloading stopped!")
	quitRadioTrackUpdate := make(chan int, 1)
	go func() {
		firstTime := true
		log.Println("Starting Radio track update")
		defer log.Println("Stopped Radio track update")
		for {
			select {
			case <-quitRadioTrackUpdate:
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
				case <-quitRadioTrackUpdate:
					return
				case metadata = <-c:
				}
			}
			if atomic.LoadInt32(&s.isRadioStreaming) > 0 {
				pos := int64(s.vorbisEncoder.GranulePos())
				atomic.StoreInt64(&s.startPos, pos)
				s.deltaChannel <- pos
				s.setTrack(metadata)
			}
		}
	}()
	for {
		stream, err := s.radioTrack.Download()
		if err != nil {
			continue
		}
		if s.encodeRadio(stream, quit) {
			break
		}
	}
	quitRadioTrackUpdate <- 1
}
func (s *Server) processRadio(quit chan int) {
	time.Sleep(time.Until(s.lastStreamEnded))
	atomic.StoreInt32(&s.isRadioStreaming, 1)
	defer atomic.StoreInt32(&s.isRadioStreaming, 0)
	quitPreload := make(chan int, 10)
	s.radioTrack.InitWS()
	s.currentTrack = s.radioTrack
	go s.preloadRadio(quitPreload)
	defer log.Println("Radio stream ended")
	defer s.radioTrack.CloseWS()
	defer func() { log.Println("Resuming track streaming...") }()
	s.lastStreamEnded = s.streamToClients(quit, quitPreload)
}
