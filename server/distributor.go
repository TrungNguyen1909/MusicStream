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

func (s *Server) pushPCMAudio(pcm []byte) {
	s.bufferingChannel <- chunk{buffer: pcm}
}
func (s *Server) pushSilentFrames() {
	silenceBuffer := make([]byte, 76032)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			s.pushPCMAudio(silenceBuffer)
		}
	}
}
func (s *Server) endCurrentStream() {
	s.bufferingChannel <- chunk{buffer: nil, encoderTime: 0}
}
func (s *Server) streamVorbis(encodedDuration chan time.Duration) chan chunk {
	var encodedTime time.Duration
	var bufferedTime time.Duration
	source := make(chan chunk, 5000)
	go func() {
		start := time.Now()
		for {
			var Chunk chunk
			select {
			case <-encodedDuration:
				for len(source) > 0 {
					select {
					case <-source:
					default:
					}
				}
				encodedDuration <- bufferedTime
				return
			case Chunk = <-source:
			}
			if Chunk.buffer == nil {
				encodedDuration <- bufferedTime
				return
			}
			output := make([]byte, 20000)
			n := s.vorbisEncoder.Encode(output, Chunk.buffer)
			output = output[:n]
			encodedTime += (time.Duration)(len(Chunk.buffer)/4/48) * time.Millisecond
			if n > 0 {
				done := false
				Chunk = chunk{}
				Chunk.buffer = output
				Chunk.channel = ((s.currentVorbisChannel + 1) % 2)
				for !done {
					select {
					case c := <-s.vorbisChannel[s.currentVorbisChannel]:
						select {
						case c <- Chunk:
						default:
						}
					default:
						s.currentVorbisChannel = (s.currentVorbisChannel + 1) % 2
						done = true
					}
				}
				bufferedTime = encodedTime
				time.Sleep(bufferedTime - time.Since(start))
			}
		}
	}()
	return source
}
func (s *Server) streamMP3(encodedDuration chan time.Duration) chan chunk {
	var encodedTime time.Duration
	var bufferedTime time.Duration
	source := make(chan chunk, 5000)
	go func() {
		start := time.Now()
		for {
			var Chunk chunk
			select {
			case <-encodedDuration:
				for len(source) > 0 {
					select {
					case <-source:
					default:
					}
				}
				encodedDuration <- bufferedTime
				return
			case Chunk = <-source:
			}
			if Chunk.buffer == nil {
				encodedDuration <- bufferedTime
				return
			}
			output := make([]byte, 20000)
			n := s.mp3Encoder.Encode(output, Chunk.buffer)
			output = output[:n]
			encodedTime += (time.Duration)(len(Chunk.buffer)/4/48) * time.Millisecond
			if n > 0 {
				done := false
				Chunk = chunk{}
				Chunk.buffer = output
				Chunk.channel = ((s.currentMP3Channel + 1) % 2)
				for !done {
					select {
					case c := <-s.mp3Channel[s.currentMP3Channel]:
						select {
						case c <- Chunk:
						default:
						}
					default:
						s.currentMP3Channel = (s.currentMP3Channel + 1) % 2
						done = true
					}
				}
				bufferedTime = encodedTime
				time.Sleep(bufferedTime - time.Since(start))
			}
		}
	}()
	return source
}
func (s *Server) streamToClients(quit chan int, quitPreload chan int) time.Time {
	s.streamMux.Lock()
	defer s.streamMux.Unlock()
	start := time.Now()
	interrupted := false
	quitVorbis := make(chan time.Duration)
	quitMP3 := make(chan time.Duration)
	vorbisStream := s.streamVorbis(quitVorbis)
	mp3Stream := s.streamMP3(quitMP3)
	for {
		select {
		case <-quit:
			quitVorbis <- time.Duration(0)
			quitMP3 <- time.Duration(0)
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
			vorbisStream <- Chunk
			mp3Stream <- Chunk
			if Chunk.buffer == nil {
				log.Println("Found last chunk, breaking...")
				break
			}
		} else {
			for {
				Chunk := <-s.bufferingChannel
				if Chunk.buffer == nil {
					log.Println("Found last chunk, breaking...")
					break
				}
			}
			break
		}
	}
	var vorbisTime, mp3Time time.Duration
	for !interrupted {
		select {
		case <-quit:
			quitVorbis <- time.Duration(0)
			quitMP3 <- time.Duration(0)
			for len(quit) > 0 {
				select {
				case <-quit:
				default:
				}
			}
			quit = nil
		case vorbisTime = <-quitVorbis:
			select {
			case mp3Time = <-quitMP3:
				interrupted = true
			}
		}
	}
	streamTime := vorbisTime
	if vorbisTime < mp3Time {
		streamTime = mp3Time
	}
	log.Println("streamTime: ", streamTime)
	return start.Add(streamTime)
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
