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
	"github.com/TrungNguyen1909/MusicStream/common"
)

func (s *Server) enqueueCallback(value interface{}) {
	track := value.(common.Track)
	metadata := common.GetMetadata(track)
	s.cacheQueue.Push(metadata)
	data := Response{
		Operation: opTrackEnqueued,
		Success:   true,
		Data: map[string]interface{}{
			"track": metadata,
		},
	}
	s.webSocketAnnounce(data.EncodeJSON())
}
func (s *Server) dequeueCallback(value interface{}) {
	removed := s.cacheQueue.Pop().(common.TrackMetadata)
	data := Response{
		Operation: opClientRemoveTrack,
		Success:   true,
		Data: map[string]interface{}{
			"track":  removed,
			"silent": true,
		},
	}
	s.webSocketAnnounce(data.EncodeJSON())
}
