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
	"encoding/json"
	"log"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/gorilla/websocket"
)

func (s *Server) pushPCMAudio(pcm []byte, encodedTime *time.Duration) {
	output := make([]byte, 20000)
	encodedLen := s.encoder.Encode(output, pcm)
	output = output[:encodedLen]
	*encodedTime += (time.Duration)(len(pcm)/4/48) * time.Millisecond
	if len(output) > 0 {
		s.bufferingChannel <- chunk{buffer: output, encoderTime: *encodedTime}
	}
}
func (s *Server) pushSilentFrames(encodedTime *time.Duration) {
	silenceBuffer := make([]byte, 76032)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			s.pushPCMAudio(silenceBuffer, encodedTime)
		}
	}
}
func (s *Server) endCurrentStream() {
	s.bufferingChannel <- chunk{buffer: nil, encoderTime: 0}
}
func (s *Server) streamToClients(quit chan int, quitPreload chan int) {
	s.streamMux.Lock()
	defer s.streamMux.Unlock()
	start := time.Now()
	s.etaDone.Store(start)
	interrupted := false
	for {
		select {
		case <-quit:
			quitPreload <- 0
			interrupted = true
			for len(quit) > 0 {
				select {
				case <-quit:
				default:
				}
			}
		default:
		}
		if !interrupted {
			Chunk := <-s.bufferingChannel
			if Chunk.buffer == nil {
				log.Println("Found last chunk, breaking...")
				break
			}
			done := false
			Chunk.channel = ((s.currentChannel + 1) % 2)
			for !done {
				select {
				case c := <-s.channels[s.currentChannel]:
					select {
					case c <- Chunk:
					default:
					}
				default:
					s.currentChannel = (s.currentChannel + 1) % 2
					done = true
				}
			}
			s.etaDone.Store(start.Add(Chunk.encoderTime))
			time.Sleep(Chunk.encoderTime - time.Since(start) - chunkDelayMS*time.Millisecond)
		} else {
			for {
				Chunk := <-s.bufferingChannel
				if Chunk.buffer == nil {
					log.Println("Found last chunk, breaking...")
					break
				}
			}
			return
		}
	}
}
func (s *Server) setTrack(trackMeta common.TrackMetadata) {
	s.currentTrackMeta = trackMeta
	log.Printf("Setting track on all clients %v - %v\n", trackMeta.Title, trackMeta.Artist)
	data, _ := json.Marshal(map[string]interface{}{
		"op":        opSetClientsTrack,
		"track":     trackMeta,
		"pos":       <-s.deltaChannel,
		"listeners": atomic.LoadInt32(&s.listenersCount),
	})
	s.webSocketAnnounce(data)
}
func (s *Server) setListenerCount() {
	data, _ := json.Marshal(map[string]interface{}{
		"op":        opSetClientsListeners,
		"listeners": atomic.LoadInt32(&s.listenersCount),
	})
	s.webSocketAnnounce(data)
}
func (s *Server) webSocketAnnounce(msg []byte) {
	s.connections.Range(func(key, value interface{}) bool {
		ws := value.(*webSocket)
		ws.WriteMessage(websocket.TextMessage, msg)
		return true
	})
}
