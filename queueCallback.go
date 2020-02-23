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
	"encoding/json"
	"log"

	"github.com/TrungNguyen1909/MusicStream/common"
	"github.com/gorilla/websocket"
	_ "github.com/joho/godotenv/autoload"
)

func enqueueCallback(value interface{}) {
	track := value.(common.Track)
	metadata := common.GetMetadata(track)
	cacheQueue.Enqueue(metadata)
	go func(metadata common.TrackMetadata) {
		log.Printf("Enqueuing track on all clients %v - %v\n", metadata.Title, metadata.Artist)
		data, err := json.Marshal(map[string]interface{}{
			"op":    opTrackEnqueued,
			"track": metadata,
		})
		connections.Range(func(key, value interface{}) bool {
			ws := value.(*webSocket)
			if err != nil {
				return false
			}
			ws.WriteMessage(websocket.TextMessage, data)
			return true
		})
	}(metadata)
}
func dequeueCallback() {
	cacheQueue.Dequeue()
}
