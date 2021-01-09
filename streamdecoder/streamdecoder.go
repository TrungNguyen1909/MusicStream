/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2020 Nguyễn Hoàng Trung(TrungNguyen1909)
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

package streamdecoder

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/acomagu/bufpipe"
	"github.com/ebml-go/webm"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
	"github.com/pkg/errors"
	"gopkg.in/hraban/opus.v2"
)

//MP3Decoder represents a mp3 decoding stream
type MP3Decoder struct {
	s beep.StreamSeekCloser
	r *beep.Resampler
	f beep.Format
}

func (decoder *MP3Decoder) Read(p []byte) (n int, err error) {
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
			return 0, fmt.Errorf("encode: invalid precision: %d", decoder.f.Precision)
		}
	}
	n = len(samples) * decoder.f.Width()
	return
}

//Close closes the mp3 stream and the underlying stream
func (decoder *MP3Decoder) Close() (err error) {
	return decoder.s.Close()
}

//NewMP3Decoder returns a 16bit/48khz PCM stream with provided mp3 stream
func NewMP3Decoder(stream io.ReadCloser) (decoder *MP3Decoder, err error) {
	streamer, format, err := mp3.Decode(stream)
	if err != nil {
		return
	}
	var resampler *beep.Resampler
	if format.SampleRate != beep.SampleRate(48000) {
		resampler = beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
		format.SampleRate = beep.SampleRate(48000)
	}
	decoder = &MP3Decoder{s: streamer, r: resampler, f: format}
	return
}

//VorbisDecoder represents a vorbis decoding stream
type VorbisDecoder struct {
	s beep.StreamSeekCloser
	r *beep.Resampler
	f beep.Format
}

func (decoder *VorbisDecoder) Read(p []byte) (n int, err error) {
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
			return 0, fmt.Errorf("encode: invalid precision: %d", decoder.f.Precision)
		}
	}
	n = len(samples) * decoder.f.Width()
	return
}

//Close closes the Vorbis stream and the underlying stream
func (decoder *VorbisDecoder) Close() (err error) {
	return decoder.s.Close()
}

//NewVorbisDecoder returns a 16bit/48khz PCM stream with provided vorbis stream
func NewVorbisDecoder(stream io.ReadCloser) (decoder *VorbisDecoder, err error) {
	streamer, format, err := vorbis.Decode(stream)
	if err != nil {
		return
	}
	var resampler *beep.Resampler
	if format.SampleRate != beep.SampleRate(48000) {
		resampler = beep.Resample(4, format.SampleRate, beep.SampleRate(48000), streamer)
		format.SampleRate = beep.SampleRate(48000)
	}
	decoder = &VorbisDecoder{s: streamer, r: resampler, f: format}
	return
}

//OpusDecoder represents a opus decoding stream
type OpusDecoder struct {
	s      io.Reader
	o      *opus.Decoder
	buffer bytes.Buffer
	err    error
}

func (decoder *OpusDecoder) Read(p []byte) (n int, err error) {
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
			return
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

//Close closes the opus decoding stream and the underlying stream
func (decoder *OpusDecoder) Close() (err error) {
	rc, ok := decoder.s.(io.ReadCloser)
	if ok {
		err = rc.Close()
	}
	return
}

//NewOpusDecoder returns a new 16bit/48khz PCM stream with the provided opus stream
func NewOpusDecoder(stream io.Reader, sampleRate, channels int) (decoder *OpusDecoder, err error) {
	os, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return
	}
	return &OpusDecoder{s: stream, o: os}, nil
}

//WebMDecoder represents a WebM decoding stream.
//
//BUG(TrungNguyen1909): The WebMDecoder will buffer the whole stream until close because webm requires a seekable stream.
type WebMDecoder struct {
	br     *bufpipe.PipeReader
	bw     *bufpipe.PipeWriter
	reader *webm.Reader
	meta   webm.WebM
	o      *opus.Decoder
	s      io.ReadSeeker
	atrack *webm.TrackEntry
}

//Close closes the webm decoding stream and the underlying stream
func (decoder *WebMDecoder) Close() (err error) {
	decoder.reader.Shutdown()
	if closer, ok := decoder.s.(io.Closer); ok {
		err = closer.Close()
	}
	return
}
func (decoder *WebMDecoder) Read(p []byte) (n int, err error) {
	return decoder.br.Read(p)
}

//NewWebMDecoder returns a new 16bit/48khz PCM audio stream with the provided WebM stream
//
//BUG(TrungNguyen1909): Only WebM stream with opus audio is supported.
func NewWebMDecoder(stream io.ReadCloser) (decoder *WebMDecoder, err error) {
	var meta webm.WebM
	src, ok := stream.(io.ReadSeeker)
	if !ok {
		src = &BufferedReadSeeker{r: stream}
	}
	reader, err := webm.Parse(src, &meta)
	if err != nil {
		return
	}
	atrack := meta.FindFirstAudioTrack()
	if atrack == nil || atrack.CodecID != "A_OPUS" {
		err = errors.WithStack(errors.New("Failed to get audio from webm/audio codec unsupported"))
		return
	}
	br, bw := bufpipe.New(nil)
	o, err := opus.NewDecoder(int(atrack.SamplingFrequency), int(atrack.Channels))
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	go func() {
		pcm := make([]int16, 5760)
		buf := make([]byte, 5760)
		for pkt := range decoder.reader.Chan {
			if pkt.TrackNumber == decoder.atrack.TrackNumber {
				n, err := o.Decode(pkt.Data, pcm)
				if err != nil {
					log.Printf("[StreamDecoder] WebMDecoder: %v", err)
					break
				}
				for i := 0; i < int(n*2); i++ {
					buf[2*i] = byte(pcm[i])
					buf[2*i+1] = byte(pcm[i] >> 8)
				}
				decoder.bw.Write(buf[:4*n])
			}
			if pkt.Timecode == webm.BadTC || pkt.Timecode == webm.BadTC*2 {
				break
			}
		}
		decoder.bw.Close()
	}()
	return &WebMDecoder{
		s:      src,
		reader: reader,
		meta:   meta,
		atrack: atrack,
		br:     br,
		bw:     bw,
		o:      o,
	}, nil
}

//BufferedReadSeeker represents a buffered seekable buffer which allows io.ReadCloser to be seeked.
//BufferedReadSeeker will read the stream as needed and keep it in memory until closed and does not support io.SeekEnd
type BufferedReadSeeker struct {
	r   io.ReadCloser
	buf bytes.Buffer
	cur int64
	len int
	err error
}

//Seek seeks BufferedReadSeeker to the provided location, io.SeekEnd is not supported
func (s *BufferedReadSeeker) Seek(offset int64, whence int) (npos int64, err error) {
	if offset == 0 && whence == io.SeekCurrent {
		return s.cur, nil
	}
	npos = s.cur
	var np int64
	switch whence {
	case io.SeekEnd:
		err = errors.WithStack(errors.New("SeekEnd not supported on BufferedReadSeeker"))
		return
	case io.SeekCurrent:
		np = s.cur + offset
	case io.SeekStart:
		np = offset
	}
	if np < 0 {
		err = errors.WithStack(errors.New("Invalid seek"))
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
func (s *BufferedReadSeeker) Read(p []byte) (n int, err error) {
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

//Close closes the underlying ReadCloser
func (s *BufferedReadSeeker) Close() (err error) {
	err = s.r.Close()
	s.err = io.EOF
	return
}
