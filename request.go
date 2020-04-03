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

package main

import (
	"encoding/json"
	"log"
	"sync/atomic"
	"time"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/csn"
	"github.com/TrungNguyen1909/MusicStream/youtube"
	_ "github.com/joho/godotenv/autoload"
)

func getPlaying() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"op":        opSetClientsTrack,
		"track":     currentTrackMeta,
		"pos":       atomic.LoadInt64(&startPos),
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	return data
}

func getListenersCount() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"op":        opSetClientsListeners,
		"listeners": atomic.LoadInt32(&listenersCount),
	})
	return data
}

func enqueue(msg wsMessage) []byte {
	var err error
	if len(msg.Query) == 0 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": false,
			"reason":  "Invalid Query!",
		})
		return data
	}
	var tracks []common.Track
	log.Printf("Client Queried: %s", msg.Query)
	switch msg.Selector {
	case common.CSN:
		tracks, err = csn.Search(msg.Query)
	case common.Youtube:
		tracks, err = youtube.Search(msg.Query)
	default:
		tracks, err = dzClient.SearchTrack(msg.Query, "")
	}
	switch {
	case err != nil:
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": false,
			"reason":  "Search Failed!",
		})
		return data
	case len(tracks) == 0:
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": false,
			"reason":  "No Result!",
		})
		return data
	default:
		track := tracks[0]
		err = track.Populate()
		if err != nil {
			log.Println("track.Populate() failed:", err)
			data, _ := json.Marshal(map[string]interface{}{
				"op":      opClientRequestTrack,
				"success": false,
				"reason":  "Search Failed!",
			})
			return data
		}
		playQueue.Enqueue(track)
		enqueueCallback(track)
		log.Printf("Track enqueued: %v - %v\n", track.Title(), track.Artist())
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestTrack,
			"success": true,
			"reason":  "",
			"track":   common.GetMetadata(track),
		})
		return data
	}
}

func getQueue() []byte {
	elements := cacheQueue.GetElements()
	tracks := make([]common.TrackMetadata, len(elements))
	for i, val := range elements {
		tracks[i] = val.(common.TrackMetadata)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"op":    opClientRequestQueue,
		"queue": tracks,
	})

	return data
}

func removeTrack(msg wsMessage) []byte {
	removed := playQueue.Remove(func(value interface{}) bool {
		ele := value.(common.Track)
		if ele.PlayID() == msg.Query {
			return true
		}
		return false
	})
	var removedTrack common.TrackMetadata
	if removed != nil {
		removedTrack = cacheQueue.Remove(func(value interface{}) bool {
			ele := value.(common.TrackMetadata)
			if ele.PlayID == msg.Query {
				return true
			}
			return false
		}).(common.TrackMetadata)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"op":      opClientRemoveTrack,
		"success": removed != nil,
		"track":   removedTrack,
	})
	if removed != nil {
		webSocketAnnounce(data)
	}
	return data
}

func skip() []byte {
	if atomic.LoadInt32(&isRadioStreaming) == 1 {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestSkip,
			"success": false,
			"reason":  "You can't skip a radio stream.",
		})

		return data
	}
	if time.Since(startTime) < 5*time.Second {
		data, _ := json.Marshal(map[string]interface{}{
			"op":      opClientRequestSkip,
			"success": false,
			"reason":  "Please wait until first 5 seconds has passed.",
		})
		return data
	}
	skipChannel <- 0
	log.Println("Current song skipped!")
	data, err := json.Marshal(map[string]interface{}{
		"op": opAllClientsSkip,
	})
	if err == nil {
		webSocketAnnounce(data)
	}
	data, _ = json.Marshal(map[string]interface{}{
		"op":      opClientRequestSkip,
		"success": true,
		"reason":  "",
	})
	return data
}
