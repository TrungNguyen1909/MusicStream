# Stage 1: Build server binary
FROM golang:alpine as build-env

WORKDIR /go/src/github.com/TrungNguyen1909/MusicStream
RUN apk --no-cache add --virtual .build-deps build-base ca-certificates git pkgconfig tzdata libogg-dev libvorbis-dev opus-dev opusfile-dev lame-dev

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

## Build the main server
RUN GOOS=linux GOARCH=amd64 go build -a --ldflags "-w -s -X github.com/TrungNguyen1909/MusicStream.BuildVersion=`git describe --tags` -X github.com/TrungNguyen1909/MusicStream.BuildTime=`date +%FT%T%z`" -v -o /bin/MusicStream cmd/MusicStream/main.go

## Build source plugins
RUN GOOS=linux GOARCH=amd64 make

# Stage 2: Build frontend
FROM node:14 AS frontend

COPY frontend /MusicStream/frontend

WORKDIR /MusicStream/frontend

RUN yarn && yarn --prod --frozen-lockfile build 

# Stage 3: Build final image
FROM alpine AS final
RUN apk --no-cache add ca-certificates tzdata libogg libvorbis opus opusfile lame
COPY --from=build-env /bin/MusicStream /bin/MusicStream
COPY --from=build-env /go/src/github.com/TrungNguyen1909/MusicStream/plugins/csn/csn.plugin plugins/csn/csn.plugin
COPY --from=build-env /go/src/github.com/TrungNguyen1909/MusicStream/plugins/youtube/youtube.plugin plugins/youtube/youtube.plugin
COPY --from=frontend /MusicStream/frontend/dist www
ENTRYPOINT ["/bin/MusicStream"]
EXPOSE 8080 
