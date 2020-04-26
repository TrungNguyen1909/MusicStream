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

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <lame/lame.h>
#define min(a,b) ((a)<(b)?(a):(b))
struct GoSlice{
	void* data;
	long len;
	long cap;
};
struct tEncoderState
{
    lame_t gfp;    
    long bitrate;
	int packet_id;
	int rate;
	int num_channels;
	int sample_rate;
	int64_t granulepos;

	long encoded_max_size;
	long encoded_length;

	int hasHeader;

	unsigned char* encoded_buffer;
};
typedef struct tEncoderState Encoder;

static Encoder* encoder_start(int sample_rate, long bitrate)
{
	Encoder *state = calloc(1,sizeof(struct tEncoderState));
	
	state->sample_rate = sample_rate;
	state->num_channels = 2;
	state->encoded_buffer = malloc(3 * 1024 * 1024);
    state->bitrate = bitrate;
	
	state->encoded_max_size = 0;
	state->encoded_length = 0;
	state->gfp = lame_init();
    lame_set_in_samplerate(state->gfp, 48000);
	lame_set_out_samplerate(state->gfp,48000);
    lame_set_VBR_max_bitrate_kbps(state->gfp, 256);
	lame_set_VBR_mean_bitrate_kbps(state->gfp, 256);
	lame_set_VBR_min_bitrate_kbps(state->gfp, 256);
    if (lame_init_params(state->gfp)!=0){
        printf("encoder_start(): Failed to initialize mp3 lame.\n");
        abort();
    }
	state->encoded_length = lame_encode_flush(state->gfp,state->encoded_buffer,3*1024*1024);
	state->hasHeader = 1;
	return state;
}

static long encode(Encoder* state, char* outputSlice, char* inputSlice){
	struct GoSlice* outSlice=(struct GoSlice*)outputSlice;
	struct GoSlice* dataSlice=(struct GoSlice*)inputSlice;
	char* out = (char*)outSlice->data;
	char* pcm = (char*)dataSlice->data;
	long out_size = outSlice->len;
	long data_size = dataSlice->len;
	if(data_size==0){
		memcpy(out,state->encoded_buffer,state->encoded_length);
		long ret = state->encoded_length;
		state->encoded_length = 0;
		return ret;
	}
	long ret =  lame_encode_buffer_interleaved(state->gfp,(short*)pcm,data_size/4,(unsigned char*)out,out_size);
	return ret;
}
static long encoder_finish(Encoder* state, char* outputSlice)
{
	struct GoSlice* outSlice=(struct GoSlice*)outputSlice;
	char* out = outSlice!=NULL?(char*)outSlice->data:0;
	long out_size = outSlice->len;
	//printf("encoder_finish(); ending stream\n");

	// write an end-of-stream packet
    out_size = lame_encode_flush(state->gfp,(unsigned char*)out,out_size);
	lame_close(state->gfp);
	free(state->encoded_buffer);
	free(state);
	return out_size;
}