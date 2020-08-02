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
	"errors"
	"log"
	"sync/atomic"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/TrungNguyen1909/MusicStream/radio"
)

func getPlaying(s *Server, msg wsMessage) Response {
	return Response{
		Operation: opSetClientsTrack,
		Success:   true,
		Data: map[string]interface{}{
			"track":     s.currentTrackMeta.Load().(common.TrackMetadata),
			"pos":       atomic.LoadInt64(&s.startPos),
			"listeners": atomic.LoadInt32(&s.listenersCount),
		},
	}
}

func getListenersCount(s *Server, msg wsMessage) Response {
	return Response{
		Operation: opSetClientsListeners,
		Success:   true,
		Data: map[string]interface{}{
			"listeners": atomic.LoadInt32(&s.listenersCount),
		},
	}
}

func enqueue(s *Server, msg wsMessage) Response {
	var err error
	if len(msg.Query) == 0 {
		return Response{
			Operation: opClientRequestTrack,
			Success:   false,
			Reason:    "Invalid Query!",
		}
	}
	var tracks []common.Track
	log.Printf("Client Queried: %s", msg.Query)
	switch msg.Selector {
	case common.CSN:
		if s.csnClient != nil {
			tracks, err = s.csnClient.Search(msg.Query)
		} else {
			err = errors.New("[CSN] Source not configured")
		}
	case common.Youtube:
		if s.ytClient != nil {
			tracks, err = s.ytClient.Search(msg.Query)
		} else {
			err = errors.New("[YT] Source not configured")
		}
	default:
		if s.dzClient != nil {
			tracks, err = s.dzClient.SearchTrack(msg.Query, "")
		} else {
			err = errors.New("[DZ] Source not configured")
		}
	}
	switch {
	case err != nil:
		log.Println("SearchTrack Failed:", err)
		return Response{
			Operation: opClientRequestTrack,
			Success:   false,
			Reason:    "Search Failed!",
		}
	case len(tracks) <= 0:
		return Response{
			Operation: opClientRequestTrack,
			Success:   false,
			Reason:    "No Result!",
		}
	default:
		track := tracks[0]
		err = track.Populate()
		if err != nil {
			log.Println("track.Populate() failed:", err)
			return Response{
				Operation: opClientRequestTrack,
				Success:   false,
				Reason:    "Search Failed!",
			}
		}
		s.playQueue.Enqueue(track)
		log.Printf("Track enqueued: %v - %v\n", track.Title(), track.Artist())
		return Response{
			Operation: opClientRequestTrack,
			Success:   true,
			Reason:    "",
			Data: map[string]interface{}{
				"track": common.GetMetadata(track),
			},
		}
	}
}

func getQueue(s *Server, msg wsMessage) Response {
	elements := s.cacheQueue.GetElements()
	tracks := make([]common.TrackMetadata, len(elements))
	for i, val := range elements {
		tracks[i] = val.(common.TrackMetadata)
	}
	return Response{
		Operation: opClientRequestQueue,
		Success:   true,
		Reason:    "",
		Data: map[string]interface{}{
			"queue": tracks,
		},
	}
}

func removeTrack(s *Server, msg wsMessage) Response {
	removed := s.playQueue.Remove(func(value interface{}) bool {
		ele := value.(common.Track)
		return ele.PlayID() == msg.Query
	})
	var removedTrack common.TrackMetadata
	if removed != nil {
		removedTrack = s.cacheQueue.Remove(func(value interface{}) bool {
			ele := value.(common.TrackMetadata)
			return ele.PlayID == msg.Query
		}).(common.TrackMetadata)
	}
	resp := Response{
		Operation: opClientRemoveTrack,
		Success:   removed != nil,
		Data: map[string]interface{}{
			"track": removedTrack,
		},
	}
	if removed != nil {
		s.webSocketAnnounce(resp.EncodeJSON())
	}
	return resp
}

func skip(s *Server, msg wsMessage) Response {
	if _, ok := s.currentTrack.(*radio.Track); ok {
		return Response{
			Operation: opClientRequestSkip,
			Success:   false,
			Reason:    "Please enqueue",
		}
	}
	if _, ok := s.currentTrack.(*common.DefaultTrack); ok {
		return Response{
			Operation: opClientRequestSkip,
			Success:   false,
			Reason:    "Please enqueue",
		}
	}
	s.skipChannel <- 0
	log.Println("Current song skipped!")
	s.webSocketAnnounce((&Response{
		Operation: opAllClientsSkip,
		Success:   true,
		Reason:    "Requested by client",
	}).EncodeJSON())
	return Response{
		Operation: opClientRequestSkip,
		Success:   true,
	}
}
func clientKeepAlivePing(s *Server, msg wsMessage) Response {
	return Response{
		Operation: opWebSocketKeepAlive,
		Success:   true,
	}
}
