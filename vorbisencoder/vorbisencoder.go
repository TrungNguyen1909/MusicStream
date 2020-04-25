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

package vorbisencoder

/*
#include "encoder.c"
#cgo pkg-config: ogg vorbis vorbisenc
*/
import "C"
import (
	"sync"
	"unsafe"
)

type Encoder struct {
	encoder *C.struct_tEncoderState
	mux     sync.Mutex
}

func NewEncoder(channels int32, sampleRate int32, bitRate uint) *Encoder {
	encoder := &Encoder{}
	encoder.encoder = (*C.struct_tEncoderState)(C.encoder_start(C.int(sampleRate), C.long(bitRate)))
	return encoder
}

func (encoder *Encoder) Encode(out []byte, data []byte) int {
	encoder.mux.Lock()
	defer encoder.mux.Unlock()
	return int(C.encode((*C.struct_tEncoderState)(encoder.encoder), (*C.char)(unsafe.Pointer(&out)), (*C.char)(unsafe.Pointer(&data))))
}
func (encoder *Encoder) EndStream(out []byte) int {
	encoder.mux.Lock()
	defer encoder.mux.Unlock()
	return int(C.encoder_finish((*C.struct_tEncoderState)(encoder.encoder), (*C.char)(unsafe.Pointer(&out))))
}

func (encoder *Encoder) GranulePos() int {
	encoder.mux.Lock()
	defer encoder.mux.Unlock()
	return int((*C.struct_tEncoderState)(encoder.encoder).granulepos)
}
