/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020  Nguyễn Hoàng Trung
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/ebml-go/webm"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"gopkg.in/hraban/opus.v2"
)

type mp3Decoder struct {
	s beep.StreamSeekCloser
	r *beep.Resampler
	f beep.Format
}

func (decoder *mp3Decoder) Read(p []byte) (n int, err error) {
	samples := make([][2]float64, len(p)/decoder.f.Width())
	var ok bool

	if decoder.r != nil {
		n, ok = decoder.r.Stream(samples)
	} else {
		n, ok = decoder.s.Stream(samples)
	}
	if !ok {
		err = io.EOF
		return
	}
	for i, sample := range samples {
		switch {
		case decoder.f.Precision == 1:
			decoder.f.EncodeUnsigned(p[i*decoder.f.Width():], sample)
		case decoder.f.Precision == 2 || decoder.f.Precision == 3:
			decoder.f.EncodeSigned(p[i*decoder.f.Width():], sample)
		default:
			panic(fmt.Errorf("encode: invalid precision: %d", decoder.f.Precision))
		}
	}
	n = len(samples) * decoder.f.Width()
	return
}
func (decoder *mp3Decoder) Close() (err error) {
	return decoder.s.Close()
}
func NewMP3Decoder(stream io.ReadCloser) (decoder *mp3Decoder, err error) {
	streamer, format, err := mp3.Decode(stream)
	if err != nil {
		return
	}
	var resampler *beep.Resampler
	if format.SampleRate != beep.SampleRate(48000) {
		resampler = beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
		format.SampleRate = beep.SampleRate(48000)
	}
	decoder = &mp3Decoder{s: streamer, r: resampler, f: format}
	return
}

type opusDecoder struct {
	s         io.Reader
	o         *opus.Decoder
	frameSize int
	buffer    bytes.Buffer
	err       error
}

func (decoder *opusDecoder) Read(p []byte) (n int, err error) {
	defer func() {
		decoder.err = err
	}()
	if decoder.err != nil {
		return 0, decoder.err
	}
	pcm := make([]int16, 5760)
	buf := make([]byte, 5760)

	d := make([]byte, 1440)
	for decoder.buffer.Len() < len(p) {
		n, err = decoder.s.Read(d)
		if err != nil {
			break
		}
		n, err = decoder.o.Decode(d[:n], pcm)
		if err != nil {
			log.Println("decoder.Decoder: ", err)
		}
		for i := 0; i < int(n*2); i++ {
			buf[2*i] = byte(pcm[i])
			buf[2*i+1] = byte(pcm[i] >> 8)
		}
		decoder.buffer.Write(buf[:4*n])
		if err != nil {
			break
		}
	}
	n, err = decoder.buffer.Read(p)
	return
}
func (decoder *opusDecoder) Close() (err error) {
	rc, ok := decoder.s.(io.ReadCloser)
	if ok {
		err = rc.Close()
	}
	return
}
func NewOpusDecoder(stream io.Reader, sampleRate, channels int) (decoder *opusDecoder, err error) {
	log.Printf("Initializing Opus Decoder: %d Hz, %d channels", sampleRate, channels)
	os, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		log.Println("DecoderCreate failed(): ", err)
	}

	if err != nil {
		return
	}
	return &opusDecoder{s: stream, o: os}, nil
}

type webmDecoder struct {
	br          *io.PipeReader
	bw          *io.PipeWriter
	reader      *webm.Reader
	meta        webm.WebM
	o           *opusDecoder
	s           io.ReadCloser
	atrack      *webm.TrackEntry
	initialized bool
}

func (decoder *webmDecoder) Close() (err error) {
	decoder.reader.Shutdown()
	decoder.o.Close()
	return decoder.s.Close()
}
func (decoder *webmDecoder) preload() {
	log.Println("Starting YT preloading")
	decoder.initialized = true
	for pkt := range decoder.reader.Chan {
		if pkt.TrackNumber == decoder.atrack.TrackNumber {
			decoder.bw.Write(pkt.Data)
		}
		if pkt.Timecode == webm.BadTC || pkt.Timecode == webm.BadTC*2 {
			break
		}
	}
	decoder.bw.Close()
	log.Println("Stopped YT preloading")
}
func (decoder *webmDecoder) Read(p []byte) (n int, err error) {
	if !decoder.initialized {
		go decoder.preload()
	}
	return decoder.o.Read(p)

}
func NewWebMDecoder(stream io.ReadCloser) (decoder *webmDecoder, err error) {
	var meta webm.WebM
	src := &source{r: stream}
	reader, err := webm.Parse(src, &meta)
	if err != nil {
		return
	}
	atrack := meta.FindFirstAudioTrack()
	if atrack != nil {
		log.Print("webm: found audio track: ", atrack.CodecID)
	}
	br, bw := io.Pipe()
	o, err := NewOpusDecoder(br, int(atrack.SamplingFrequency), int(atrack.Channels))
	if err != nil {
		log.Panic("webDecoder:Read() -> NewOpusDecoder() failed: ", err)
	}
	log.Println("Opus Decoder Created")
	return &webmDecoder{
		s:      stream,
		reader: reader,
		meta:   meta,
		atrack: atrack,
		br:     br,
		bw:     bw,
		o:      o,
	}, nil
}

type source struct {
	r   io.ReadCloser
	buf bytes.Buffer
	cur int64
	len int
	err error
}

func (s *source) Seek(offset int64, whence int) (npos int64, err error) {
	npos = s.cur
	var np int64
	switch whence {
	case io.SeekEnd:
		log.Panic("SeekEnd not supported on source")
	case io.SeekCurrent:
		np = s.cur + offset
	case io.SeekStart:
		np = offset
	}
	if np < 0 {
		err = errors.New("Invalid seek")
		return
	} else if np > int64(s.len) {
		_, err = s.Read(make([]byte, np-int64(s.len)))
	}
	if err == nil {
		s.cur = np
	}
	npos = s.cur
	return
}
func (s *source) Read(p []byte) (n int, err error) {
	defer func() {
		if err != nil {
			log.Println("source.Read failed ", err)
		}
	}()
	if s.cur+int64(len(p)) > int64(s.len) && s.err == nil {
		nb := make([]byte, s.cur+int64(len(p))-int64(s.len))
		var n int
		n, s.err = io.ReadAtLeast(s.r, nb, len(nb))
		nb = nb[:n]
		s.buf.Write(nb)
		s.len = s.buf.Len()
	}
	n = copy(p, s.buf.Bytes()[s.cur:])
	if n < len(p) || s.err != nil {
		if s.err == nil {
			s.err = io.EOF
		}
		err = s.err
	}
	s.cur += int64(n)
	return
}

func (s *source) Close() (err error) {
	err = s.r.Close()
	s.err = io.EOF
	return
}
