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
#include <assert.h>
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/opt.h>
#include <libswresample/swresample.h>

#define min(a, b) ((a) < (b) ? (a) : (b))

struct GoSlice {
	void *data;
	long long len;
	long long cap;
};

typedef int (*read_callback)(void *opaque, void *buf, int buf_size);

typedef struct Decoder {
    void *opaque;
    read_callback read_cb;
    AVIOContext *input_ioctx;
    AVFormatContext *container;
    AVCodecContext *ctx;
    SwrContext *swr_ctx;
    AVPacket *packet;
    AVFrame *frame;
    char *decoded_buffer;
    char *decoded_ptr;
    char *decoded_end;
    int stream_id;
} Decoder;

static int decoder_read(Decoder *dec, char *outputSlice)
{
    struct GoSlice *outSlice = (struct GoSlice *)outputSlice;
    char *out = outSlice->data;
    int len = outSlice->len;

    int copy_len = min(len, dec->decoded_end - dec->decoded_ptr);
    memcpy(out, dec->decoded_ptr, copy_len);
    dec->decoded_ptr += copy_len;
    out += copy_len;
    len -= copy_len;

    while (len > 0 && av_read_frame(dec->container, dec->packet) == 0) {
        if (dec->packet->stream_index == dec->stream_id) {
            if (avcodec_send_packet(dec->ctx, dec->packet) < 0) {
                break;
            }
            int ret = avcodec_receive_frame(dec->ctx, dec->frame);
            if (ret == AVERROR(EAGAIN)) {
                continue;
            } else if (ret == AVERROR_EOF) {
                break;
            } else if (ret < 0) {
                break;
            }
            int out_samples = av_rescale_rnd(swr_get_delay(dec->swr_ctx, dec->ctx->sample_rate)
                                             + dec->frame->nb_samples, 48000, dec->ctx->sample_rate,
                                             AV_ROUND_UP);
            uint8_t *output = NULL;
            if (av_samples_alloc(&output, NULL, 2, out_samples, AV_SAMPLE_FMT_S16, 1) < 0) {
                break;
            }
            out_samples = swr_convert(dec->swr_ctx, &output, out_samples,
                                      (const unsigned char **)dec->frame->extended_data,
                                      dec->frame->nb_samples);
            int dst_bufsize = av_samples_get_buffer_size(NULL, 2, out_samples, AV_SAMPLE_FMT_S16, 1);
            int copy_len = min(dst_bufsize, len);
            memcpy(out, output, copy_len);
            out += copy_len;
            len -= copy_len;
            if (copy_len < dst_bufsize) {
                assert(dst_bufsize - copy_len <= 16384);
                memcpy(dec->decoded_buffer, output + copy_len, dst_bufsize - copy_len);
                dec->decoded_ptr = dec->decoded_buffer;
                dec->decoded_end = dec->decoded_ptr + dst_bufsize - copy_len;
            }
            av_freep(&output);
        }
    }
    return outSlice->len - len;
}

static int decoder_in(void *opaque, uint8_t *buf, int buf_size)
{
    int ret = -1;
    Decoder *dec = (Decoder *)opaque;
    ret = dec->read_cb(dec->opaque, buf, buf_size);
    fflush(stdout);
    return ret;
}

static Decoder *decoder_new(void *opaque, read_callback read_cb)
{
    Decoder *dec = (Decoder *)calloc(1, sizeof(Decoder));
    if (!dec) {
        goto cleanup_1;
    }
    dec->opaque = opaque;
    dec->read_cb = read_cb;
    unsigned char *fileStreamBuffer = (unsigned char*)av_malloc(4096);
    if (!fileStreamBuffer) {
        goto cleanup_2;
    }
    AVIOContext *input_ioctx = avio_alloc_context(fileStreamBuffer, 4096, 
                                                  0, dec, decoder_in, NULL, NULL);
    if (!input_ioctx) {
        goto cleanup_3;
    }
    AVFormatContext *container = avformat_alloc_context();
    if (!container) {
        goto cleanup_4;
    }
    container->pb = input_ioctx;
    container->flags |= AVFMT_FLAG_CUSTOM_IO;
    if (avformat_open_input(&container, NULL, NULL, NULL) < 0) {
        goto cleanup_4;
    }
    if (avformat_find_stream_info(container, NULL) < 0) {
        goto cleanup_5;
    }
    int stream_id = -1;
    for (int i = 0; i < container->nb_streams; i++) {
        if (container->streams[i]->codecpar->codec_type == AVMEDIA_TYPE_AUDIO) {
            stream_id = i;
            break;
        }
    }
    if (stream_id == -1) {
        goto cleanup_5;
    }

    AVCodecParameters *codec_par = container->streams[stream_id]->codecpar;
	AVCodec *codec = avcodec_find_decoder(codec_par->codec_id);
    if (!codec) {
        goto cleanup_5;
    }
	AVCodecContext *ctx = avcodec_alloc_context3(codec);
    if (!ctx) {
        goto cleanup_5;
    }

    if (codec_par->extradata && codec_par->extradata_size > 0) {
        ctx->extradata = (uint8_t *)av_calloc(1, codec_par->extradata_size + AV_INPUT_BUFFER_PADDING_SIZE);
        if (!ctx->extradata) {
            goto cleanup_6;
        }
        memcpy(ctx->extradata, codec_par->extradata, codec_par->extradata_size);
        ctx->extradata_size = codec_par->extradata_size;
    }

    if (avcodec_open2(ctx, codec, NULL) < 0) {
        goto cleanup_6;
    }

    SwrContext *swr_ctx = swr_alloc();
    if (!swr_ctx) {
        goto cleanup_6;
    }
    av_opt_set_int(swr_ctx, "in_channel_layout",
		av_get_default_channel_layout(codec_par->channels), 0);
	av_opt_set_int(swr_ctx, "in_sample_rate",
		codec_par->sample_rate, 0);
	av_opt_set_int(swr_ctx, "in_sample_fmt",
		codec_par->format, 0);
	av_opt_set_int(swr_ctx, "out_channel_layout",
		av_get_default_channel_layout(2), 0);
	av_opt_set_int(swr_ctx, "out_sample_rate",
		48000, 0);
	av_opt_set_int(swr_ctx, "out_sample_fmt",
		AV_SAMPLE_FMT_S16, 0);

    if (swr_init(swr_ctx) < 0) {
        goto cleanup_7;
    }

    dec->input_ioctx = input_ioctx;
    dec->container = container;
    dec->ctx = ctx;
    dec->stream_id = stream_id;
    dec->packet = av_packet_alloc();
    if (!dec->packet) {
        goto cleanup_7;
    }
    dec->frame = av_frame_alloc();
    if (!dec->frame) {
        goto cleanup_8;
    }
    dec->swr_ctx = swr_ctx;
    dec->decoded_buffer = dec->decoded_ptr = dec->decoded_end = (char *)av_malloc(16384);
    if (!dec->decoded_buffer) {
        goto cleanup_9;
    }
    return dec;
cleanup_9:
    av_frame_free(&dec->frame);
cleanup_8:
    av_packet_free(&dec->packet);
cleanup_7:
    swr_free(&swr_ctx);
cleanup_6:
    if (ctx->extradata) {
        av_freep(&ctx->extradata);
    }
    avcodec_free_context(&ctx);
cleanup_5:
    avformat_close_input(&container);
cleanup_4:
    av_freep(&input_ioctx->buffer);
    avio_context_free(&input_ioctx);
    goto cleanup_2;
cleanup_3:
    av_freep(&fileStreamBuffer);
cleanup_2:
    free(dec);
cleanup_1:
    return NULL;
}

static void decoder_close(Decoder *dec)
{
    av_freep(&dec->decoded_buffer);
    av_frame_free(&dec->frame);
    av_packet_free(&dec->packet);
    swr_free(&dec->swr_ctx);
    if (dec->ctx->extradata) {
        av_freep(&dec->ctx->extradata);
    }
    avcodec_free_context(&dec->ctx);
    avformat_close_input(&dec->container);
    av_freep(&dec->input_ioctx->buffer);
    avio_context_free(&dec->input_ioctx);
    free(dec);
}
