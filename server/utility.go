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
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
)

func (s *Server) selfPinger() {
	appName, ok := os.LookupEnv("HEROKU_APP_NAME")
	if !ok {
		return
	}
	log.Println("[MusicStream] Starting periodic keep-alive ping...")
	url := fmt.Sprintf("https://%s.herokuapp.com", appName)
	for {
		if atomic.LoadInt32(&s.listenersCount) > 0 {
			resp, err := http.Get(url)
			if err != nil {
				resp.Body.Close()
			}
		}
		time.Sleep(1 * time.Minute)
	}
}

func (s *Server) listenerMonitor(ch chan int32) {
	timer := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-s.newListenerC:
			ch <- 1
		case <-timer.C:
			if listeners := atomic.LoadInt32(&s.listenersCount); listeners > 0 {
				ch <- listeners
			}
		}
	}
}

func (s *Server) inactivityMonitor() {
	timer := time.NewTimer(15 * time.Minute)
	lch := make(chan int32)
	go s.listenerMonitor(lch)
	isStandby := false
	for {
		select {
		case <-lch:
			timer.Reset(15 * time.Minute)
			if isStandby {
				log.Println("[MusicStream] Waking up...")
				s.streamMux.Unlock()
				s.activityWg.Done()
				isStandby = false
			}
		case <-timer.C:
			log.Println("[MusicStream] Inactivity. Standby...")
			isStandby = true
			s.activityWg.Add(1)
			if s.skipFunc != nil {
				s.skipFunc()
			}
			s.streamMux.Lock()
			s.updateStartPos(true)
			s.setTrack(common.GetMetadata(s.defaultTrack))
		}
	}
}
