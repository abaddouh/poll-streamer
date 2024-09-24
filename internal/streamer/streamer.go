package streamer

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

func (s *Streamer) createFIFO(streamPath string) (string, error) {
	fifoPath := filepath.Join(streamPath, "input_fifo")
	if _, err := os.Stat(fifoPath); os.IsNotExist(err) {
		if err := syscall.Mkfifo(fifoPath, 0666); err != nil {
			return "", fmt.Errorf("failed to create FIFO: %v", err)
		}
	}
	return fifoPath, nil
}

func (s *Streamer) startPersistentFFmpeg(fifoPath, streamPath string) (*exec.Cmd, error) {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "image2pipe",
		"-i", fifoPath,
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
		filepath.Join(streamPath, "stream.m3u8"),
	)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start FFmpeg: %v\nOutput: %s", err, stderr.String())
	}

	// Monitor FFmpeg process
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("FFmpeg process for %s exited with error: %v\nOutput: %s", streamPath, err, stderr.String())
		} else {
			log.Printf("FFmpeg process for %s exited successfully.", streamPath)
		}
		s.mu.Lock()
		delete(s.activeStreams, streamPath)
		s.mu.Unlock()
	}()

	return cmd, nil
}

func (s *Streamer) ProcessImage(imagePath, streamID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	streamPath := filepath.Join(s.outputPath, streamID)
	os.MkdirAll(streamPath, os.ModePerm)

	fifoPath, err := s.createFIFO(streamPath)
	if err != nil {
		log.Printf("Error creating FIFO: %v", err)
		return
	}

	// Start FFmpeg if not already running
	if _, exists := s.activeStreams[streamPath]; !exists {
		cmd, err := s.startPersistentFFmpeg(fifoPath, streamPath)
		if err != nil {
			log.Printf("Error starting FFmpeg: %v", err)
			return
		}
		s.activeStreams[streamPath] = cmd
	}

	// Write the new image to the FIFO
	fifoFile, err := os.OpenFile(fifoPath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		log.Printf("Error opening FIFO for writing: %v", err)
		return
	}
	defer fifoFile.Close()

	imgFile, err := os.Open(imagePath)
	if err != nil {
		log.Printf("Error opening image file: %v", err)
		return
	}
	defer imgFile.Close()

	if _, err := io.Copy(fifoFile, imgFile); err != nil {
		log.Printf("Error writing image to FIFO: %v", err)
	}
}

func (s *Streamer) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for streamPath, cmd := range s.activeStreams {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			log.Printf("Error sending interrupt to FFmpeg for %s: %v", streamPath, err)
		}
		// Optionally wait or implement force kill after a timeout
	}
}
