FROM golang

RUN apt-get update && apt-get install -y libogg-dev libvorbis-dev

WORKDIR /go/src/github.com/TrungNguyen1909/MusicStream

COPY . .

RUN go get -d -v ./...

RUN go install -v ./...

ENTRYPOINT ["MusicStream"]

EXPOSE 8890
