# API

## Session ID

- The `sessionId` cookie should be fetch by perform any request/endpoint to the server.
- It is used to match a stream and its corresponding websocket connection
## Stream

If you need access to raw audio stream, you can access them at
| Type   | Path      | Quality                        |
| ------ | ----------|--------------------------------|
| Vorbis | /audio    | 320kbps CBR (with stream time) |
| MP3    | /fallback | 320kbps VBR                    |

It is encouraged to use the Vorbis stream because it has the best quality and contains timestamp data for synced lyrics.

All tracks will be prepended and appended 1.584 seconds of silence. Thus, there's a 3.168 seconds of silence between two consecutive tracks.

Each session can only have 1 audio stream. Whenever a new stream is established with the same `sessionId` cookie, the old stream will be disconnected.

## Websocket

Path: `/status`

All requests and notifications on changes are delivered through websocket in JSON,
Most of the requests have a fallback HTTP API

### Structure

#### MusicSourceInfo

```go
type MusicSourceInfo struct {
	//Name is the full name of the source
	Name string `json:"name"`
	//DisplayName is the shortened name of the source, used to display on search bar
	DisplayName string `json:"display_name"`
	//ID is the source's id, assigned by the server, used for querying tracks
	ID int `json:"id"`
}
```

#### TrackMetadata

```go
type TrackMetadata struct {
	//Title is the title/name of the track
	Title      string       `json:"title"`
	//IsRadio is a bollean, whether or not the current track is radio/silent stream
	IsRadio    bool         `json:"is_radio"`
	//Duration is he duration of the track, zero if unknown
	Duration   int          `json:"duration"`
	//Artist is the main artist/channel(Youtube) of the track
	Artist     string       `json:"artist"`
	//Artists is a list of artists/contributors/channel(Youtube) of the track, comma-separated
	Artists    string       `json:"artists"`
	//Album is the album of the track, if known
	Album      string       `json:"album"`
	//CoverURL contains an URL to the cover art/thumbnail of the track, if known
	CoverURL   string       `json:"cover"`
	//Lyrics contains information about the track lyrics, if known
	Lyrics     LyricsResult `json:"lyrics"`
	//PlayID is an unique ID for every track
	PlayID     string       `json:"playId"`
	//SpotifyURI is the Spotify URI that track in Spotify, if known
	SpotifyURI string       `json:"spotifyURI"`
	//ID is the ID of the track from the source
	ID         string       `json:"id"`
	//Href is the link to the track
	Href       string       `json:"href"`
}
```

#### LyricsResult

```go
//LyricsTime represents the time that the lyrics will be shown, counted from the start of the track
type LyricsTime struct {
	Hundredths int     `json:"hundredths"`
	Minutes    int     `json:"minutes"`
	Seconds    int     `json:"seconds"`
	Total      float64 `json:"total"`
}

//LyricsLine contains informations about a piece of lyrics
type LyricsLine struct {
	Text       string     `json:"text"`
	Translated string     `json:"translated"`
	Time       LyricsTime `json:"time"`
	Original   string     `json:"original"`
}

//LyricsResult represents a result of a lyrics query
type LyricsResult struct {
	RawLyrics    string       `json:"txt"`
	SyncedLyrics []LyricsLine `json:"lrc"`
	Language     string       `json:"lang"`
}
```

### Opcode

```js
opListSources         = 1
opSetClientsTrack     = 2
opAllClientsSkip      = 3
opClientRequestTrack  = 4
opClientRequestSkip   = 5
opSetClientsListeners = 6
opTrackEnqueued       = 7
opClientRequestQueue  = 8
opWebSocketKeepAlive  = 9
opClientRemoveTrack   = 10
opClientAudioStartPos = 11
```

### Requests

All the requests should be a JSON-encoded dictionary, contains at least the key `op`, whose value is the integer equivalent to the request's opcode.

It's recommended that clients should set the `nonce` key to the message to a randomly generated non-zero number to find the response and prevent duplication of action.

### Response/Notifications

The server will send a message, structured as below as an JSON-encoded dictionary to clients.

```go
type Response struct {
	//Operation is the message's opcode. Dictionary key is "op"
	Operation int                    `json:"op"`
	//Success specifies whether the request succeeded or not, it will always be true in case of a notification
	Success   bool                   `json:"success"`
	//Reason specifies the reason for the value of Success
	Reason    string                 `json:"reason"`
	//Data is a dictionary, containing any data associated with the message
	Data      map[string]interface{} `json:"data"`
	//Nonce is a integer, unique to every requests
	Nonce     int                    `json:"nonce"`
}
```

### Opcode description

Description for each opcodes, the equivalent REST API path is surrounded in parentheses.

#### opListSources (/sources)
- Clients send this opcode to request list of sources the server supported.
- The response message from the server will be a list of `MusicSourceInfo` in the key `sources` of the `data` dictionary.

#### opSetClientsTrack (/playing)
- Clients send a request containing this opcode to get the current playing track
- This message will be sent automatically by the server upon connection
- When receive this opcode from the websocket, either the server responds to the inquiry or the server has just played a different track.
- Data will contain the following keys:
    - track: a TrackMetadata object containing the metadata of the playing track
    - pos: The audio frame number where the track starts. You can get the time in seconds using the following expression: `pos / 48000.0 + 1.584`.
    - listeners: The number of clients connected to the stream.

#### opClientRequestTrack (/enqueue)
- Clients send this opcode in a message structured like below to enqueue a track
    - query is the search query
    - selector is a `MusicSourceInfo`'s id, the source from which a track is fetched from
```go
type trackRequestMessage struct {
	Operation int    `json:"op"`
	Query     string `json:"query"`
	Selector  int    `json:"selector"`
	Nonce     int    `json:"nonce"`
}
```
- The server will respond to the request in a message that contains the same opcode and nonce specifies whether the request succeeded or not.
#### opAllClientsSkip (Notification only)
- The server sends this opcode when the current playing track is skipped by a client

#### opClientRequestSkip (/skip)
- Clients send this opcode to request the server to skip the track that is currently been played
- The server will respond to the request in a message that contains the same opcode and nonce specifies whether the request succeeded or not.

#### opSetClientsListeners (/listeners)
- Clients send this opcode to request the number of clients connected to the stream.
- The response message from the server will be in the key `listeners` of the `data` dictionary.

#### opTrackEnqueued (Notification only)
- A new track has just been added to the queue. The track's simplifed metadata is in the `track` key of the `Data` dictionary structured as `TrackMetadata`.

#### opClientRequestQueue (/queue)
- Clients send this message to request the current track queue.
- This message will be sent automatically by the server upon connection
- The server will answer the queue in the key `queue` of the `data` dictionary as an array of `TrackMetadata`.

#### opWebSocketKeepAlive (Ping)
- A ping should be sent every 30-45 seconds to prevent the websocket being terminated due to inactivity, especially if the server is hosted on Heroku.
- The server will respond the message with the same opcode.

#### opClientRemoveTrack (/remove)
- The client send this message in the following structure

```go
type trackRemoveRequestMessage struct {
	Operation int    `json:"op"`
	Query     string `json:"query"`
	Nonce     int    `json:"nonce"`
}
```

- The key `query` should contains the `playID` of the track that should be removed from the queue.
- The server will respond in a message which contains the same `op` and `nonce` describes whether the removal is successful or not.
- The server will send this message to all clients in case of a successful removal. The track being removed is in the key `track` in the `data` dictionary. The `playID` field should be used to distinguish between tracks. The key `silent` will be `true` if the removal was initiated by this client, in this case, the UI should displayed to its user that the track has been removed successfully. Otherwise, the UI may remove the track in discretion.

### opClientAudioStartPos (Notification)
- Uses to show synced lyrics.
- The key `startPos` should contains the number of the first audio frame sent to the audio stream that contains the same `sessionId` cookie string, divide by `48000.0` to get the time in second.
- `startPos` should be added to your audio player's current time only if the player does NOT parse the position data of the Vorbis stream.
	- Among browsers, only Chromium-based browsers seem to parse the position data

- The notification will be sent when the websocket connection is established or when an audio stream with the same `sessionId` starts to send audio data.