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
	"log"
	"sync/atomic"

	"github.com/TrungNguyen1909/MusicStream/common"
)

func getSourcesList(s *Server, msg wsMessage) Response {
	result := make([]common.MusicSourceInfo, len(s.sources))
	for i, v := range s.sources {
		result[i] = common.GetMusicSourceInfo(v)
		result[i].ID = i
	}
	return Response{
		Operation: opListSources,
		Success:   true,
		Data: map[string]interface{}{
			"sources": result,
		},
	}
}
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
	if msg.Selector < 0 || msg.Selector >= len(s.sources) {
		return Response{
			Operation: opClientRequestTrack,
			Success:   false,
			Reason:    "Invalid source!",
		}
	}
	log.Printf("[MusicStream] Client Queried: Source: %s: %s", s.sources[msg.Selector].Name(), msg.Query)
	tracks, err = s.sources[msg.Selector].Search(msg.Query)
	switch {
	case err != nil:
		log.Printf("[MusicStream] SearchTrack: Source: %s: Failed: %v", s.sources[msg.Selector].Name(), err)
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
			log.Printf("[MusicStream] track.Populate() failed: %+v", err)
			return Response{
				Operation: opClientRequestTrack,
				Success:   false,
				Reason:    "Search Failed!",
			}
		}
		s.playQueue.Push(track)
		log.Printf("[MusicStream] Track enqueued: %v - %v\n", track.Title(), track.Artist())
		return Response{
			Operation: opClientRequestTrack,
			Success:   true,
			Data: map[string]interface{}{
				"track": common.GetMetadata(track),
			},
		}
	}
}

func getQueue(s *Server, msg wsMessage) Response {
	elements := s.cacheQueue.Values()
	tracks := make([]common.TrackMetadata, len(elements))
	for i, val := range elements {
		tracks[i] = val.(common.TrackMetadata)
	}
	return Response{
		Operation: opClientRequestQueue,
		Success:   true,
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
	if !resp.Success {
		resp.Reason = "Failed to remove track"
	}
	if removed != nil {
		s.webSocketNotify(resp)
	}
	return resp
}

func skip(s *Server, msg wsMessage) Response {
	if s.skipFunc == nil || s.streamContext.Err() != nil {
		return Response{
			Operation: opClientRequestSkip,
			Success:   false,
			Reason:    "There's no track to be skipped",
		}
	}
	s.skipFunc()
	log.Println("[MusicStream] Current song skipped!")
	s.webSocketNotify(Response{
		Operation: opAllClientsSkip,
		Success:   true,
		Reason:    "Requested by client",
	})
	return Response{
		Operation: opClientRequestSkip,
		Success:   true,
	}
}
func clientKeepAlivePing(s *Server, msg wsMessage) Response {
	s.newListenerC <- 1
	return Response{
		Operation: opWebSocketKeepAlive,
		Success:   true,
	}
}
