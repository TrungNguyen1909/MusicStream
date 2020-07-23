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
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//Config contains the API keys for Server
type Config struct {
	DeezerARL             string
	MusixMatchUserToken   string
	MusixMatchOBUserToken string
	YoutubeDeveloperKey   string
	SpotifyClientID       string
	SpotifyClientSecret   string
	RadioEnabled          bool
}
type chunk struct {
	buffer      []byte
	encoderTime time.Duration
	channel     int
	chunkID     int64
}
type wsMessage struct {
	Operation int    `json:"op"`
	Query     string `json:"query"`
	Selector  int    `json:"selector"`
	Nonce     int    `json:"nonce"`
}

type webSocket struct {
	conn *websocket.Conn
	mux  *sync.Mutex
}

func (socket *webSocket) WriteMessage(messageType int, data []byte) error {
	socket.mux.Lock()
	defer socket.mux.Unlock()
	return socket.conn.WriteMessage(messageType, data)
}
func (socket *webSocket) Close() error {
	socket.mux.Lock()
	defer socket.mux.Unlock()
	return socket.conn.Close()
}
func (socket *webSocket) ReadJSON(v interface{}) error {
	return socket.conn.ReadJSON(v)
}
func (socket *webSocket) ReadMessage() (messageType int, p []byte, err error) {
	return socket.conn.ReadMessage()
}

//Response will be encode in JSON and send to user
type Response struct {
	Operation int                    `json:"op"`
	Success   bool                   `json:"success"`
	Reason    string                 `json:"reason"`
	Data      map[string]interface{} `json:"data"`
	Nonce     int                    `json:"nonce"`
}

//EncodeJSON encodes Response to JSON
func (r Response) EncodeJSON() []byte {
	encoded, _ := json.Marshal(r)
	return encoded
}

//RequestHandler is a function that handles a request from user.
type RequestHandler func(s *Server, msg wsMessage) Response
