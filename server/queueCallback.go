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

	"github.com/TrungNguyen1909/MusicStream/common"
)

func (s *Server) enqueueCallback(value interface{}) {
	track := value.(common.Track)
	metadata := common.GetMetadata(track)
	s.cacheQueue.Enqueue(metadata)
	log.Printf("Enqueuing track on all clients %v - %v\n", metadata.Title, metadata.Artist)
	data, _ := json.Marshal(map[string]interface{}{
		"op":    opTrackEnqueued,
		"track": metadata,
	})
	s.webSocketAnnounce(data)
}
func (s *Server) dequeueCallback() {
	removed := s.cacheQueue.Pop().(common.TrackMetadata)
	data, _ := json.Marshal(map[string]interface{}{
		"op":      opClientRemoveTrack,
		"success": true,
		"track":   removed,
		"silent":  true,
	})
	s.webSocketAnnounce(data)
}
