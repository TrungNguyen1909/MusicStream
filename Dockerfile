FROM golang as build-env

WORKDIR /go/src/github.com/TrungNguyen1909/MusicStream

RUN apt-get update && apt-get install -y libogg-dev libvorbis-dev

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -v -o ./MusicStream

ENTRYPOINT ["./MusicStream"]
EXPOSE 8890
