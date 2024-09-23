package streamer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Streamer struct {
	outputPath     string
	frameRate      int
	resolution     string
	bitrate        string
	placeholderImg string
	activeStreams  map[string]*exec.Cmd
	mu             sync.Mutex
}

func New(outputPath string, frameRate int, resolution, bitrate, placeholderImg string) *Streamer {
	return &Streamer{
		outputPath:     outputPath,
		frameRate:      frameRate,
		resolution:     resolution,
		bitrate:        bitrate,
		placeholderImg: placeholderImg,
		activeStreams:  make(map[string]*exec.Cmd),
	}
}

func (s *Streamer) ProcessImage(imagePath, streamID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cmd, exists := s.activeStreams[streamID]; exists {
		log.Printf("Stopping existing stream for %s", streamID)
		if err := cmd.Process.Kill(); err != nil {
			log.Printf("Error stopping stream %s: %v", streamID, err)
		}
		delete(s.activeStreams, streamID)
	}

	streamPath := filepath.Join(s.outputPath, streamID)
	os.MkdirAll(streamPath, os.ModePerm)

	cmd := s.startFFmpeg(imagePath, streamPath)
	s.activeStreams[streamID] = cmd
}

func (s *Streamer) startFFmpeg(imagePath, streamPath string) *exec.Cmd {
	cmd := exec.Command("ffmpeg",
		"-re",
		"-f", "image2",
		"-loop", "1",
		"-i", imagePath,
		"-vf", fmt.Sprintf("fps=%d", s.frameRate),
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+append_list",
		"-codec:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-s", s.resolution,
		"-b:v", s.bitrate,
		"-maxrate", s.bitrate,
		"-bufsize", s.bitrate,
		"-max_muxing_queue_size", "1024",
		filepath.Join(streamPath, "stream.m3u8"))

	var stderr strings.Builder
	cmd.Stderr = &stderr

	go func() {
		err := cmd.Run()
		if err != nil {
			log.Printf("FFmpeg error: %v\nFFmpeg output:\n%s", err, stderr.String())
		}
	}()

	return cmd
}

func (s *Streamer) InitializePlaceholderStream(streamID string) {
	s.ProcessImage(s.placeholderImg, streamID)
}
