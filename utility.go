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
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	_ "github.com/joho/godotenv/autoload"
)

func selfPinger() {
	appName, ok := os.LookupEnv("HEROKU_APP_NAME")
	if !ok {
		return
	}
	log.Println("Starting periodic keep-alive ping...")
	url := fmt.Sprintf("https://%s.herokuapp.com", appName)
	for {
		if atomic.LoadInt32(&listenersCount) > 0 {
			http.Get(url)
			log.Println("Ping!")
		}
		time.Sleep(1 * time.Minute)
	}
}

func listenerMonitor(ch chan int32) {
	timer := time.NewTimer(1 * time.Minute)
	for {
		if listeners := atomic.LoadInt32(&listenersCount); listeners > 0 {
			ch <- listeners
		}
		timer.Reset(1 * time.Minute)
		select {
		case <-newListenerC:
		case <-timer.C:
		}
	}
}

func inactivityMonitor() {
	timer := time.NewTimer(15 * time.Minute)
	lch := make(chan int32)
	go listenerMonitor(lch)
	isStandby := false
	for {
		select {
		case l := <-lch:
			timer.Reset(15 * time.Minute)
			if isStandby {
				log.Println("Waking up...")
				if playQueue.Empty() {
					go processRadio(quitRadio)
				}
				activityWg.Done()
				isStandby = false
			}
			log.Println("Listeners: ", l)
		case <-timer.C:
			log.Println("Inactivity. Standby...")
			isStandby = true
			activityWg.Add(1)
			if atomic.LoadInt32(&isRadioStreaming) > 0 {
				quitRadio <- 0
				streamMux.Lock()
				streamMux.Unlock()
				<-quitRadio
			} else {
				skipChannel <- 1
			}
			deltaChannel <- 0
			setTrack(common.TrackMetadata{
				Title:   "Standby...",
				Artist:  "Inactivity",
				Artists: "The Stream is standby due to inactivity",
			})
		}
	}
}
