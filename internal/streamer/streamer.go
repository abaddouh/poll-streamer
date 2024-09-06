package streamer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Streamer struct {
	outputPath string
	frameRate  int
	resolution string
	bitrate    string
}

func New(outputPath string, frameRate int, resolution, bitrate string) *Streamer {
	return &Streamer{
		outputPath: outputPath,
		frameRate:  frameRate,
		resolution: resolution,
		bitrate:    bitrate,
	}
}

func (s *Streamer) ProcessImage(imagePath string) {
	os.MkdirAll(s.outputPath, os.ModePerm)
	s.startFFmpeg(imagePath)
}

func (s *Streamer) startFFmpeg(imagePath string) {
	cmd := exec.Command("ffmpeg",
		"-re",          // Read input at native frame rate (important for live streaming)
		"-f", "image2", // Force input format to image2
		"-loop", "1", // Loop the input image indefinitely
		"-i", imagePath, // Input file
		"-vf", fmt.Sprintf("fps=%d", s.frameRate), // Set the output frame rate
		"-f", "hls", // Force output format to HTTP Live Streaming (HLS)
		"-hls_time", "2", // Set the target segment length in seconds
		"-hls_list_size", "5", // Limit the playlist length (in number of segments)
		"-hls_flags", "delete_segments+append_list", // Delete old segments and append to the existing list
		"-codec:v", "libx264", // Use H.264 video codec
		"-preset", "ultrafast", // Encoding speed preset (fastest, but larger file size)
		"-tune", "zerolatency", // Tune the encoding for zero latency streaming
		"-s", s.resolution, // Set the output resolution
		"-b:v", s.bitrate, // Set the target average bitrate
		"-maxrate", s.bitrate, // Set the maximum bitrate
		"-bufsize", s.bitrate, // Set the rate control buffer size
		"-max_muxing_queue_size", "1024", // Increase muxing queue size to avoid buffering issues
		filepath.Join(s.outputPath, "stream.m3u8")) // Output HLS playlist

	var stderr strings.Builder
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("FFmpeg error: %v\nFFmpeg output:\n%s", err, stderr.String())
	}
}
