package vorbisencoder

/*
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <vorbis/codec.h>
#include <vorbis/vorbisenc.h>
#define min(a,b) ((a)<(b)?(a):(b))
struct GoSlice{
	void* data;
	long len;
	long cap;
};
struct tEncoderState
{
	ogg_stream_state os;

	vorbis_info vi;
	vorbis_comment vc;
	vorbis_dsp_state vd;
	vorbis_block vb;
	ogg_packet op;

	int packet_id;
	int rate;
	int num_channels;
	int sample_rate;
	int granulepos;

	int encoded_max_size;
	int encoded_length;

	int hasHeader;

	unsigned char* encoded_buffer;
};
typedef struct tEncoderState Encoder;
int write_page(Encoder* state, ogg_page* page)
{
		memcpy(state->encoded_buffer + state->encoded_length, page->header, page->header_len);
		state->encoded_length += page->header_len;

		memcpy(state->encoded_buffer + state->encoded_length, page->body, page->body_len);
		state->encoded_length += page->body_len;

		//printf("write_page(); total encoded stream length: %i bytes\n", state->encoded_length);
		return page->header_len+page->body_len;
}

Encoder* encoder_start(int sample_rate)
{
	Encoder *state = calloc(1,sizeof(struct tEncoderState));
	srand(time(NULL));
	ogg_stream_init(&state->os,rand());

	state->sample_rate = sample_rate;
	state->num_channels = 2;
	state->encoded_buffer = calloc(1,3 * 1024 * 1024);
	printf("encoder_start(); initializing vorbis encoder with sample_rate = %i Hz\n", state->sample_rate);

	state->encoded_max_size = 0;
	state->encoded_length = 0;
	vorbis_info_init(&state->vi);
	// if(vorbis_encode_init_vbr(&state->vi, 2, state->sample_rate, 0.4f)){
	// 	printf("encoder_start() failed: vorbis_encoder_init_vbr()\n");
	// 	return NULL;
	// }
	if(vorbis_encode_init(&state->vi, 2, state->sample_rate, 256000, 256000, 256000)){
		printf("encoder_start() failed: vorbis_encoder_init()\n");
		return NULL;
	}
	vorbis_comment_init(&state->vc);
	vorbis_comment_add_tag(&state->vc, "Encoder", "MusicStream");
	vorbis_analysis_init(&state->vd, &state->vi);
	vorbis_block_init(&state->vd, &state->vb);
	ogg_packet header,headerComm,headerCode;
	vorbis_analysis_headerout(&state->vd, &state->vc, &header, &headerComm, &headerCode);
	ogg_stream_packetin(&state->os, &header);
	ogg_stream_packetin(&state->os, &headerComm);
	ogg_stream_packetin(&state->os, &headerCode);
	state->hasHeader = 1;
	return state;
}

long encode(Encoder* state, void* outputSlice, void* inputSlice){
	struct GoSlice* outSlice=(struct GoSlice*)outputSlice;
	struct GoSlice* dataSlice=(struct GoSlice*)inputSlice;
	char* out = (char*)outSlice->data;
	char* pcm = (char*)dataSlice->data;
	long out_size = outSlice->len;
	long data_size = dataSlice->len;
	if(state->hasHeader){
		ogg_page og;
		while(ogg_stream_flush(&state->os, &og)){
			write_page(state,&og);
		}
		state->hasHeader = 0;
	}
	if(data_size==0){
		memcpy(out,state->encoded_buffer,state->encoded_length);
		long ret = state->encoded_length;
		state->encoded_length = 0;
		return ret;
	}
	float** buffer = vorbis_analysis_buffer(&state->vd, data_size/4);
	for(int i=0;i<data_size/4;i++){
		buffer[0][i] = ((short)(pcm[4*i+1]<<8)|(short)(0x00ff&pcm[i*4]))/32768.0;
		buffer[1][i] = ((short)(pcm[4*i+3]<<8)|(short)(0x00ff&pcm[i*4+2]))/32768.0;
	}
	vorbis_analysis_wrote(&state->vd, data_size/4);
	ogg_page og;
	while (vorbis_analysis_blockout(&state->vd, &state->vb)==1){
		vorbis_analysis(&state->vb, NULL);
		vorbis_bitrate_addblock(&state->vb);
		while(vorbis_bitrate_flushpacket(&state->vd, &state->op))
		{
			// push packet into ogg
			ogg_stream_packetin(&state->os, &state->op);

			// fetch page from ogg
			while(ogg_stream_pageout(&state->os, &og) || (state->op.e_o_s && ogg_stream_flush(&state->os, &og)))
			{
				write_page(state, &og);
			}
		}
	}
	memcpy(out,state->encoded_buffer,min(state->encoded_length,out_size));
	long ret = min(state->encoded_length,out_size);
	state->encoded_length -=min(state->encoded_length,out_size);
	return ret;
}
void encoder_finish(Encoder* state)
{
	printf("encoder_finish(); ending stream\n");

	// write an end-of-stream packet
	vorbis_analysis_wrote(&state->vd, 0);

	ogg_page og;

	while(vorbis_analysis_blockout(&state->vd, &state->vb) == 1)
	{
		vorbis_analysis(&state->vb, NULL);
		vorbis_bitrate_addblock(&state->vb);

		while(vorbis_bitrate_flushpacket(&state->vd, &state->op))
		{
			ogg_stream_packetin(&state->os, &state->op);

			while(ogg_stream_flush(&state->os, &og))
				write_page(state, &og);
		}
	}

	printf("encoder_finish(); final encoded stream length: %i bytes\n", state->encoded_length);
	printf("encoder_finish(); cleaning up\n");

	ogg_stream_clear(&state->os);
	vorbis_block_clear(&state->vb);
	vorbis_dsp_clear(&state->vd);
	vorbis_comment_clear(&state->vc);
	vorbis_info_clear(&state->vi);
}

#cgo pkg-config: ogg vorbis vorbisenc
*/
import "C"
import "unsafe"

type Encoder C.struct_tEncoderState

func NewEncoder(channels int32, sampleRate int32) *Encoder {
	encoder := (*Encoder)(C.encoder_start(C.int(sampleRate)))
	return encoder
}

func (encoder *Encoder) Encode(out []byte, data []byte) int {
	return int(C.encode((*C.struct_tEncoderState)(encoder), unsafe.Pointer(&out), unsafe.Pointer(&data)))
}
func (encoder *Encoder) Close() {
	C.encoder_finish((*C.struct_tEncoderState)(encoder))
}
