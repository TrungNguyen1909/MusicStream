module github.com/TrungNguyen1909/MusicStream

go 1.13

require (
	github.com/anaskhan96/soup v1.1.1
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebml-go/ebml v0.0.0-20160925193348-ca8851a10894 // indirect
	github.com/ebml-go/webm v0.0.0-20160924163542-629e38feef2a
	github.com/faiface/beep v1.0.2
	github.com/gorilla/websocket v1.4.2
	github.com/joho/godotenv v1.3.0
	github.com/labstack/echo/v4 v4.1.17
	github.com/petar/GoLLRB v0.0.0-20190514000832-33fb24c13b99 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.20.0
	github.com/rylio/ytdl v1.0.4
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/net v0.0.0-20200904194848-62affa334b73 // indirect
	golang.org/x/sys v0.0.0-20200918174421-af09f7315aff // indirect
	golang.org/x/text v0.3.3
	gopkg.in/hraban/opus.v2 v2.0.0-20200710132758-e28f8214483b
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

replace github.com/rylio/ytdl => github.com/mihaiav/ytdl v0.6.3-0.20200510100116-5f2bf8b4fec0
