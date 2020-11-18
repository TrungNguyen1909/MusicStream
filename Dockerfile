# Stage 1: Build server binary
FROM golang:alpine as build-env

WORKDIR /go/src/github.com/TrungNguyen1909/MusicStream
RUN apk --no-cache add --virtual .build-deps build-base ca-certificates git pkgconfig tzdata libogg-dev libvorbis-dev opus-dev opusfile-dev lame-dev

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -a --ldflags "-w -s -X github.com/TrungNguyen1909/MusicStream.BuildVersion=`git describe --tags` -X github.com/TrungNguyen1909/MusicStream.BuildTime=`date +%FT%T%z`" -v -o /bin/MusicStream cmd/MusicStream/main.go

# Stage 2: Build frontend
FROM node:14 AS frontend

COPY --from=build-env /go/src/github.com/TrungNguyen1909/MusicStream/frontend /MusicStream/frontend

WORKDIR /MusicStream/frontend

RUN yarn && yarn --prod --frozen-lockfile build 

# Stage 3: Build final image
FROM alpine 
RUN apk --no-cache add ca-certificates tzdata libogg libvorbis opus opusfile lame
COPY --from=build-env /bin/MusicStream /bin/MusicStream
COPY --from=frontend /MusicStream/frontend/dist www
ENTRYPOINT ["/bin/MusicStream"]
EXPOSE 8080 
