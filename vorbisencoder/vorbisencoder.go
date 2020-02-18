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
