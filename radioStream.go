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
	"github.com/faiface/beep/vorbis"
	_ "github.com/joho/godotenv/autoload"
)

func encodeRadio(stream io.ReadCloser, encodedTime *time.Duration, quit chan int) (ended bool) {
	streamer, format, err := vorbis.Decode(stream)
	if err != nil {
		log.Println(err)
		return false
	}

	defer streamer.Close()

	samples := make([][2]float64, 960)
	buf := make([]byte, len(samples)*format.Width())
	for {
		select {
		case <-quit:
			return true
		default:
		}
		n, ok := streamer.Stream(samples)
		if !ok {
			return false
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
		pushPCMAudio(buf[:n*format.Width()], encodedTime)
		if 0 <= n && n < len(samples) && ok {
			return false
		}
	}
}
func preloadRadio(quit chan int) {
	var encodedTime time.Duration
	time.Sleep(time.Until(etaDone.Load().(time.Time)))
	log.Println("Radio preloading started!")
	defer endCurrentStream()
	defer pushSilentFrames(&encodedTime)
	defer log.Println("Radio preloading stopped!")
	quitRadioSetTrack := make(chan int, 1)
	go func(quit chan int) {
		firstTime := true
		for {
			if !firstTime {
				radioTrack.WaitForTrackUpdate()
			} else {
				firstTime = false
			}
			select {
			case <-quitRadioSetTrack:
				return
			default:
			}
			pos := int64(encoder.GranulePos())
			atomic.StoreInt64(&startPos, pos)
			deltaChannel <- pos
			setTrack(common.GetMetadata(radioTrack))
		}
	}(quitRadioSetTrack)
	stream, _ := radioTrack.Download()
	for !encodeRadio(stream, &encodedTime, quit) {
		stream, _ = radioTrack.Download()
	}
	quitRadioSetTrack <- 1
}
func processRadio(quit chan int) {
	quitPreload := make(chan int, 10)
	radioTrack.InitWS()
	time.Sleep(time.Until(etaDone.Load().(time.Time)))
	currentTrack = radioTrack
	go preloadRadio(quitPreload)
	atomic.StoreInt32(&isRadioStreaming, 1)
	defer atomic.StoreInt32(&isRadioStreaming, 0)
	defer log.Println("Radio stream ended")
	defer radioTrack.CloseWS()
	defer func() { log.Println("Resuming track streaming..."); quit <- 0 }()
	streamToClients(quit, quitPreload)
}
