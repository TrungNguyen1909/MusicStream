/*
 * MusicStream - Listen to music together with your friends from everywhere, at the same time.
 * Copyright (C) 2021 Nguyễn Hoàng Trung(TrungNguyen1909)
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

/*
#include "decoder.c"

int decoderIn(void *opaque, void *buf, int buf_size);
#cgo pkg-config: libavcodec libavformat libavutil libswresample
*/
import "C"

import (
	"bytes"
	"io"
	"unsafe"

	pointer "github.com/mattn/go-pointer"
	"github.com/pkg/errors"
)

//AVDecoder is a Audio Decoder, backed by libav
type AVDecoder struct {
	r       io.ReadCloser
	p       unsafe.Pointer
	dec     *C.struct_Decoder
	err     error
	errRead error
}

func (d *AVDecoder) Read(p []byte) (n int, err error) {
	if d.errRead != nil {
		return 0, d.errRead
	}
	n = int(C.decoder_read(d.dec, (*C.char)(unsafe.Pointer(&p))))
	if n == 0 {
		err = io.EOF
		d.errRead = err
	}
	return n, err
}

func (d *AVDecoder) Close() (err error) {
	C.decoder_close(d.dec)
	d.dec = nil
	pointer.Unref(d.p)
	return d.r.Close()
}

//export decoderIn
func decoderIn(opaque unsafe.Pointer, buf unsafe.Pointer, buf_size C.int) C.int {
	d := pointer.Restore(opaque).(*AVDecoder)
	if d.err != nil {
		return -1
	}
	buffer := (*[1 << 28]byte)(buf)[:int(buf_size):int(buf_size)]
	n, err := io.ReadFull(d.r, buffer)
	d.err = err
	return C.int(n)
}

//NewAVDecoder returns a s16le/48khz PCM stream decoded from ffmpeg
func NewAVDecoder(stream io.ReadCloser) (decoder *AVDecoder, err error) {
	decoder = &AVDecoder{}
	decoder.r = stream
	decoder.p = pointer.Save(decoder)
	decoder.dec = C.decoder_new(decoder.p, C.read_callback(C.decoderIn))
	if decoder.dec == nil {
		return nil, errors.New("Failed to initialize C decoder")
	}
	return decoder, nil
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
