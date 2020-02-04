module github.com/TrungNguyen1909/MusicStream

go 1.13

// +heroku goVersion go1.13
require (
	common v0.0.0
	csn v0.0.0
	deezer v0.0.0
	github.com/anaskhan96/soup v1.1.1
	github.com/faiface/beep v1.0.2
	github.com/gorilla/websocket v1.4.1
	github.com/hajimehoshi/go-mp3 v0.2.1
	github.com/hajimehoshi/oto v0.5.4
	github.com/jfreymuth/oggvorbis v1.0.0
	github.com/joho/godotenv v1.3.0
	github.com/pkg/errors v0.8.1
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876
	golang.org/x/exp v0.0.0-20191227195350-da58074b4299
	golang.org/x/image v0.0.0-20191214001246-9130b4cfad52
	golang.org/x/mobile v0.0.0-20191210151939-1a1fef82734d
	golang.org/x/sys v0.0.0-20200107144601-ef85f5a75ddf
	golang.org/x/text v0.3.2
	lyrics v0.0.0
	queue v0.0.0
	vorbisencoder v0.0.0
)

replace csn => ./csn

replace common => ./common

replace deezer => ./deezer

replace lyrics => ./lyrics

replace queue => ./queue

replace vorbisencoder => ./vorbisencoder
