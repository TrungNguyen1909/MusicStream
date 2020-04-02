FROM golang as build-env

WORKDIR /go/src/github.com/TrungNguyen1909/MusicStream

RUN apt-get update && apt-get install musl musl-dev musl-tools linux-headers
ENV CC="musl-gcc" STATIC_CC="musl-gcc" CCOPT="-static -fPIC" BUILDMODE="static" 
RUN curl -sqLO https://ftp.osuosl.org/pub/xiph/releases/ogg/libogg-1.3.4.tar.gz
RUN curl -sqLO https://ftp.osuosl.org/pub/xiph/releases/vorbis/libvorbis-1.3.6.tar.gz
RUN curl -sqLO https://ftp.osuosl.org/pub/xiph/releases/opus/opus-1.3.1.tar.gz
RUN curl -sqLO https://ftp.osuosl.org/pub/xiph/releases/opus/opusfile-0.11.tar.gz
RUN curl -sqLO https://www.openssl.org/source/openssl-1.1.1f.tar.gz
RUN tar -xf libogg-1.3.4.tar.gz
RUN tar -xf libvorbis-1.3.6.tar.gz
RUN tar -xf opus-1.3.1.tar.gz
RUN tar -xf opusfile-0.11.tar.gz
RUN tar -xf openssl-1.1.1f.tar.gz
RUN cd libogg-1.3.4 && ./configure && make && make install
RUN cd libvorbis-1.3.6 && ./configure && make && make install
RUN cd opus-1.3.1 && ./configure && make && make install
RUN cd openssl-1.1.1f  && ./config -fPIC -static && make depend && make && make install
ENV PKG_CONFIG_PATH=$PKG_CONFIG_PATH:/usr/local/ssl/lib/pkgconfig OPENSSL_STATIC=1
RUN cd opusfile-0.11 && ./configure && make && make install


COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_CFLAGS="-I/usr/local/musl/include/" CFLAGS="-I/usr/local/musl/include/" CGO_LDFLAGS="-w -s -L/usr/local/lib -logg -lvorbis -lvorbisenc -lopus -lopusfile" go build -a --ldflags '-w -s -linkmode external -extldflags "-static"' -v -o /bin/MusicStream .

FROM scratch
COPY --from=build-env /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-env /bin/MusicStream /bin/MusicStream
COPY --from=build-env /go/src/github.com/TrungNguyen1909/MusicStream/www www

ENTRYPOINT ["/bin/MusicStream"]
EXPOSE 8890
