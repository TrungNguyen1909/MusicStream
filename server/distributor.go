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
	"bytes"
	"context"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/gorilla/websocket"
)

func (s *Server) pushPCMAudio(pcm []byte) {
	s.bufferingChannel <- &chunk{buffer: pcm}
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
	s.bufferingChannel <- &chunk{buffer: nil}
}
func (s *Server) streamVorbis(streamContext context.Context, encodedDuration chan time.Duration) chan *chunk {
	var encodedTime time.Duration
	var bufferedTime time.Duration
	source := make(chan *chunk, 5000)
	go func() {
		start := time.Now()
		defer func() {
			encodedDuration <- bufferedTime
		}()
		for {
			var Chunk *chunk
			select {
			case <-streamContext.Done():
				for len(source) > 0 {
					<-source
				}
				return
			case Chunk = <-source:
			}
			if Chunk.buffer == nil {
				return
			}
			output := make([]byte, 20000)
			pos := s.vorbisEncoder.GranulePos()
			n := s.vorbisEncoder.Encode(output, Chunk.buffer)
			output = output[:n]
			encodedTime += (time.Duration)(len(Chunk.buffer)/4/48) * time.Millisecond
			if n > 0 {
				Chunk := &chunk{}
				Chunk.buffer = output
				Chunk.channel = ((s.currentVorbisChannel + 1) % 2)
				Chunk.chunkID = atomic.AddInt64(s.vorbisChunkID, 1)
				Chunk.encoderPos = pos
				sent := int64(0)
				for len(s.vorbisChannel[s.currentVorbisChannel]) > 0 || sent < atomic.LoadInt64(s.vorbisSubscribers) {
					c := <-s.vorbisChannel[s.currentVorbisChannel]
					c <- Chunk
					sent++
				}
				s.currentVorbisChannel = (s.currentVorbisChannel + 1) % 2
				bufferedTime = encodedTime
				time.Sleep(bufferedTime - time.Since(start))
			}
		}
	}()
	return source
}
func (s *Server) streamMP3(streamContext context.Context, encodedDuration chan time.Duration) chan *chunk {
	var encodedTime time.Duration
	var bufferedTime time.Duration
	source := make(chan *chunk, 5000)
	go func() {
		defer func() {
			encodedDuration <- bufferedTime
		}()
		var buffer bytes.Buffer
		start := time.Now()
		for {
			var Chunk *chunk
			select {
			case <-streamContext.Done():
				for len(source) > 0 {
					<-source
				}
				return
			case Chunk = <-source:
			}
			buffer.Write(Chunk.buffer)
			if buffer.Len() < 1152*4 && Chunk.buffer != nil {
				continue
			}
			var pcm []byte
			if Chunk.buffer != nil {
				pcm = make([]byte, (1152*4)*(buffer.Len()/(1152*4)))
				_, _ = buffer.Read(pcm)
			} else {
				sz := buffer.Len()
				sz += (1152*4 - sz%(1152*4))
				pcm = make([]byte, sz)
				_, _ = buffer.Read(pcm)
			}
			output := make([]byte, 20000)
			pos := s.mp3Encoder.GranulePos()
			n := s.mp3Encoder.Encode(output, pcm)
			output = output[:n]
			encodedTime += (time.Duration)(len(pcm)/4/48) * time.Millisecond
			if n > 0 {
				Chunk := &chunk{}
				Chunk.buffer = output
				Chunk.channel = ((s.currentMP3Channel + 1) % 2)
				Chunk.chunkID = atomic.AddInt64(s.mp3ChunkID, 1)
				Chunk.encoderPos = pos
				sent := int64(0)
				for len(s.mp3Channel[s.currentMP3Channel]) > 0 || sent < atomic.LoadInt64(s.mp3Subscribers) {
					c := <-s.mp3Channel[s.currentMP3Channel]
					c <- Chunk
					sent++
				}
				s.currentMP3Channel = (s.currentMP3Channel + 1) % 2
				bufferedTime = encodedTime
				time.Sleep(bufferedTime - time.Since(start))
			}
			if Chunk.buffer == nil {
				return
			}
		}
	}()
	return source
}

func (s *Server) updateStartPos(push bool) {
	pos := int64(s.vorbisEncoder.GranulePos())
	atomic.StoreInt64(&s.startPos[0], pos)
	if push {
		s.deltaChannel <- pos
	}
	pos = int64(s.mp3Encoder.GranulePos())
	atomic.StoreInt64(&s.startPos[1], pos)
	if push {
		s.deltaChannel <- pos
	}
}
func (s *Server) streamToClients(streamContext context.Context) time.Time {
	start := time.Now()
	interrupted := false
	timeVorbis := make(chan time.Duration)
	timeMP3 := make(chan time.Duration)
	vorbisStream := s.streamVorbis(streamContext, timeVorbis)
	mp3Stream := s.streamMP3(streamContext, timeMP3)
	var vorbisTime, mp3Time time.Duration
	for {
		select {
		case <-streamContext.Done():
			interrupted = true
		default:
		}
		if !interrupted {
			Chunk := <-s.bufferingChannel
			vorbisStream <- Chunk
			mp3Stream <- Chunk
			if Chunk.buffer == nil {
				break
			}
		} else {
			for {
				Chunk := <-s.bufferingChannel
				if Chunk.buffer == nil {
					break
				}
			}
			break
		}
	}
	if !interrupted {
		for !interrupted {
			select {
			case <-streamContext.Done():
			case vorbisTime = <-timeVorbis:
				mp3Time = <-timeMP3
				interrupted = true
			}
		}
	} else {
		vorbisTime = <-timeVorbis
		mp3Time = <-timeMP3
	}
	streamTime := vorbisTime
	if vorbisTime < mp3Time {
		streamTime = mp3Time
	}
	log.Println("[MusicStream] streamTime: ", streamTime)
	return start.Add(streamTime)
}

func (s *Server) setTrack(trackMeta common.TrackMetadata) {
	s.currentTrackMeta.Store(trackMeta)
	data := Response{
		Operation: opSetClientsTrack,
		Success:   true,
		Data: map[string]interface{}{
			"track":       trackMeta,
			"pos":         <-s.deltaChannel,
			"fallbackpos": <-s.deltaChannel,
			"listeners":   atomic.LoadInt32(&s.listenersCount),
		},
	}
	s.webSocketNotify(data)
}
func (s *Server) setListenerCount() {
	data := Response{
		Operation: opSetClientsListeners,
		Success:   true,
		Data: map[string]interface{}{
			"listeners": atomic.LoadInt32(&s.listenersCount),
		},
	}
	s.webSocketNotify(data)
}
func (s *Server) webSocketNotify(response Response) {
	if response.Nonce == 0 {
		response.Nonce = int(rand.Int31())
	}
	s.connections.Range(func(key, value interface{}) bool {
		ws := value.(*webSocket)
		_ = ws.WriteMessage(websocket.TextMessage, response.EncodeJSON())
		return true
	})
}
