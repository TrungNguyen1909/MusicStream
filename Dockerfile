FROM golang

RUN apt-get update && apt-get install -y libogg-dev libvorbis-dev

ADD . /go/src/github.com/TrungNguyen1909/MusicStream

RUN go install github.com/TrungNguyen1909/MusicStream

RUN cp -R /go/src/github.com/TrungNguyen1909/MusicStream/www .

ENTRYPOINT /go/bin/MusicStream

EXPOSE 8890
