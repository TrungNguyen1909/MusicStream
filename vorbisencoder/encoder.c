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

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <assert.h>
#include <vorbis/codec.h>
#include <vorbis/vorbisenc.h>
#define min(a, b) ((a) < (b) ? (a) : (b))
struct GoSlice {
	void *data;
	long long len;
	long long cap;
};
typedef struct Encoder {
	ogg_stream_state os;

	vorbis_info vi;
	vorbis_comment vc;
	vorbis_dsp_state vd;
	vorbis_block vb;
	ogg_packet op;
    
    long bitrate;
	int packet_id;
	int rate;
	int num_channels;
	int sample_rate;
	int64_t granulepos;

	unsigned char *encoded_buffer;
	unsigned char *encoded_ptr;
	unsigned char *encoded_end;
	unsigned char *encoded_max_end;
} Encoder;
static int write_page(Encoder *state, ogg_page* page)
{
	if (state->encoded_end == state->encoded_ptr) {
		state->encoded_ptr = state->encoded_end = state->encoded_buffer;
	}
	memcpy(state->encoded_end, page->header, page->header_len);
	state->encoded_end += page->header_len;

	memcpy(state->encoded_end, page->body, page->body_len);
	state->encoded_end += page->body_len;
	assert (state->encoded_end < state->encoded_max_end);
	return page->header_len + page->body_len;
}

static int out_buffer(Encoder *state, char **out, long *out_size)
{
	int copy_length = min(*out_size, state->encoded_end - state->encoded_ptr);

	memcpy(*out, state->encoded_ptr, copy_length);
	*out += copy_length;
	*out_size -= copy_length;
	state->encoded_ptr += copy_length;

	if (state->encoded_end == state->encoded_ptr) {
		state->encoded_ptr = state->encoded_end = state->encoded_buffer;
	}

	return copy_length;
}

static Encoder *encoder_start(int sample_rate, long bitrate)
{
	Encoder *state = calloc(1, sizeof(Encoder));
	srand(time(NULL));
	ogg_stream_init(&state->os, rand());

	state->sample_rate = sample_rate;
	state->num_channels = 2;
	state->encoded_buffer = state->encoded_ptr = state->encoded_end = malloc(4 * 1024 * 1024);
	state->encoded_max_end = state->encoded_buffer + (4 * 1024 * 1024);
    state->bitrate = bitrate;

	vorbis_info_init(&state->vi);
	if (vorbis_encode_init(&state->vi, 2, state->sample_rate, state->bitrate,
						   state->bitrate, state->bitrate)) {
		fprintf(stderr, "encoder_start() failed: vorbis_encoder_init()\n");
		return NULL;
	}
	vorbis_comment_init(&state->vc);
	vorbis_comment_add_tag(&state->vc, "Encoder", "MusicStream");
	vorbis_analysis_init(&state->vd, &state->vi);
	vorbis_block_init(&state->vd, &state->vb);
	ogg_packet header, headerComm, headerCode;
	vorbis_analysis_headerout(&state->vd, &state->vc, &header, &headerComm, &headerCode);
	ogg_stream_packetin(&state->os, &header);
	ogg_stream_packetin(&state->os, &headerComm);
	ogg_stream_packetin(&state->os, &headerCode);
	ogg_page og;
	while (ogg_stream_flush(&state->os, &og)) {
		write_page(state, &og);
	}
	return state;
}

static long encode(Encoder *state, char* outputSlice, char* inputSlice)
{
	long ret = 0;
	struct GoSlice *outSlice = (struct GoSlice *)outputSlice;
	struct GoSlice *dataSlice = (struct GoSlice *)inputSlice;
	char* out = (char *)outSlice->data;
	char* pcm = (char *)dataSlice->data;
	long out_size = outSlice->len;
	long data_size = dataSlice->len;
	ret += out_buffer(state, &out, &out_size);

	if (data_size > 0) {
		float **buffer = vorbis_analysis_buffer(&state->vd, data_size / 4);
		for (int i = 0; i < data_size / 4; i++) {
			buffer[0][i] = ((short)(pcm[4 * i + 1 ] << 8)|(short)(0x00ff & pcm[i * 4])) / 32768.0;
			buffer[1][i] = ((short)(pcm[4 * i + 3 ] << 8)|(short)(0x00ff & pcm[i * 4 + 2])) / 32768.0;
		}
		vorbis_analysis_wrote(&state->vd, data_size / 4);
	}
	ogg_page og;
	while (vorbis_analysis_blockout(&state->vd, &state->vb) == 1) {
		vorbis_analysis(&state->vb, NULL);
		vorbis_bitrate_addblock(&state->vb);
		while (vorbis_bitrate_flushpacket(&state->vd, &state->op) == 1) {
			// push packet into ogg
			ogg_stream_packetin(&state->os, &state->op);

			// fetch page from ogg
			while (ogg_stream_pageout(&state->os, &og)
				   || (state->op.e_o_s && ogg_stream_flush(&state->os, &og))) {
				write_page(state, &og);
				state->granulepos = ogg_page_granulepos(&og);
			}
		}
	}
	ret += out_buffer(state, &out, &out_size);
	return ret;
}
static long encoder_finish(Encoder *state, char *outputSlice)
{
	struct GoSlice *outSlice = (struct GoSlice *)outputSlice;
	char *out = (outSlice != NULL) ? (char *)outSlice->data : 0;
	long out_size = outSlice->len;

	// write an end-of-stream packet
	vorbis_analysis_wrote(&state->vd, 0);

	ogg_page og;

	while (vorbis_analysis_blockout(&state->vd, &state->vb) == 1) {
		vorbis_analysis(&state->vb, NULL);
		vorbis_bitrate_addblock(&state->vb);

		while (vorbis_bitrate_flushpacket(&state->vd, &state->op)) {
			ogg_stream_packetin(&state->os, &state->op);

			while(ogg_stream_flush(&state->os, &og)) {
				write_page(state, &og);
				state->granulepos = ogg_page_granulepos(&og);
			}
		}
	}

	long ret = out_buffer(state, &out, &out_size);

	ogg_stream_clear(&state->os);
	vorbis_block_clear(&state->vb);
	vorbis_dsp_clear(&state->vd);
	vorbis_comment_clear(&state->vc);
	vorbis_info_clear(&state->vi);
	free(state->encoded_buffer);
	free(state);
	return ret;
}
