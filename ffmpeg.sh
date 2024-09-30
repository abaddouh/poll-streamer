#!/bin/bash

ffmpeg \
  -y \
  -loglevel verbose \
  -f image2pipe \
  -analyzeduration 1000000 \
  -probesize 1000000 \
  -framerate 1 \
  -i ./stream/stream1/input_fifo \
  -c:v libx264 \
  -preset ultrafast \
  -tune zerolatency \
  -vf "scale=640:480,format=yuv420p" \
  -color_range pc \
  -colorspace bt709 \
  -color_primaries bt709 \
  -color_trc bt709 \
  -g 1 \
  -keyint_min 1 \
  -sc_threshold 0 \
  -b:v 800k \
  -maxrate 800k \
  -bufsize 1600k \
  -f hls \
  -hls_time 1 \
  -hls_list_size 3 \
  -hls_flags delete_segments+append_list+independent_segments+omit_endlist \
  -hls_segment_filename ./stream/stream1/segment%03d.ts \
  ./stream/stream1/stream.m3u8
