FROM golang as build-env

WORKDIR /go/src/github.com/TrungNguyen1909/MusicStream

RUN apt-get update && apt-get install -y libogg-dev libvorbis-dev

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -v -ldflags="-w -s" -o /bin/MusicStream .

FROM scratch
COPY --from=build-env /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-env /lib64/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2
COPY --from=build-env /usr/lib/x86_64-linux-gnu/libogg.so.0 /usr/lib/x86_64-linux-gnu/libogg.so.0
COPY --from=build-env /usr/lib/x86_64-linux-gnu/libvorbis.so.0 /usr/lib/x86_64-linux-gnu/libvorbis.so.0
COPY --from=build-env /usr/lib/x86_64-linux-gnu/libvorbisenc.so.2 /usr/lib/x86_64-linux-gnu/libvorbisenc.so.2
COPY --from=build-env /lib/x86_64-linux-gnu/libpthread.so.0 /lib/x86_64-linux-gnu/libpthread.so.0
COPY --from=build-env /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libc.so.6
COPY --from=build-env /lib/x86_64-linux-gnu/libm.so.6 /lib/x86_64-linux-gnu/libm.so.6
COPY --from=build-env /bin/MusicStream /bin/MusicStream
COPY --from=build-env /go/src/github.com/TrungNguyen1909/MusicStream/www www

ENTRYPOINT ["/bin/MusicStream"]
EXPOSE 8890
