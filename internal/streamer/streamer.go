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
	activeStreams  map[string]*StreamProcess
	mu             sync.Mutex
}

type StreamProcess struct {
	cmd      *exec.Cmd
	fifoFile *os.File
}

func New(outputPath string, frameRate int, resolution, bitrate, placeholderImg string) *Streamer {
	return &Streamer{
		outputPath:     outputPath,
		frameRate:      frameRate,
		resolution:     resolution,
		bitrate:        bitrate,
		placeholderImg: placeholderImg,
		activeStreams:  make(map[string]*StreamProcess),
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

func (s *Streamer) startPersistentFFmpeg(fifoPath, streamPath string) (*exec.Cmd, *os.File, error) {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-re",
		"-f", "image2pipe",
		"-i", fifoPath,
		"-vf", fmt.Sprintf("fps=%d", s.frameRate),
		"-g", fmt.Sprintf("%d", s.frameRate*2), // Keyframe every 2 seconds
		"-r", fmt.Sprintf("%d", s.frameRate), // Output fps
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
		return nil, nil, fmt.Errorf("failed to start FFmpeg: %v\nOutput: %s", err, stderr.String())
	}

	fifoFile, err := os.OpenFile(fifoPath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		cmd.Process.Kill()
		return nil, nil, fmt.Errorf("failed to open FIFO for writing: %v", err)
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

	return cmd, fifoFile, nil
}

func (s *Streamer) writeImageToFIFO(fifoFile *os.File, imagePath, streamID string) error {
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("error opening image file %s: %v", imagePath, err)
	}
	defer imgFile.Close()

	log.Printf("Writing image %s to FIFO", imagePath)
	_, err = io.Copy(fifoFile, imgFile)
	if err != nil {
		return fmt.Errorf("error writing image to FIFO: %v", err)
	}
	log.Printf("Successfully wrote image to FIFO for stream %s", streamID)
	return nil
}

func (s *Streamer) ProcessImage(imagePath, streamID string) {
	log.Printf("active streams: %v", s.activeStreams)
	s.mu.Lock()
	defer s.mu.Unlock()

	streamPath := filepath.Join(s.outputPath, streamID)
	log.Printf("Processing image: %s for Stream ID: %s at Path: %s", imagePath, streamID, streamPath)

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		log.Printf("Image file does not exist: %s", imagePath)
		return
	}

	if err := os.MkdirAll(streamPath, os.ModePerm); err != nil {
		log.Printf("Error creating stream directory: %v", err)
		return
	}

	fifoPath, err := s.createFIFO(streamPath)
	if err != nil {
		log.Printf("Error creating FIFO at %s: %v", fifoPath, err)
		return
	}
	log.Printf("FIFO created at: %s", fifoPath)

	// Start FFmpeg if not already running
	if _, exists := s.activeStreams[streamPath]; !exists {
		cmd, fifoFile, err := s.startPersistentFFmpeg(fifoPath, streamPath)
		if err != nil {
			log.Printf("Error starting FFmpeg for %s: %v", streamPath, err)
			return
		}
		s.activeStreams[streamPath] = &StreamProcess{
			cmd:      cmd,
			fifoFile: fifoFile,
		}
		log.Printf("Started FFmpeg for stream %s with PID %d", streamPath, cmd.Process.Pid)
	}

	// Write the new image to the FIFO
	streamProcess := s.activeStreams[streamPath]
	if err := s.writeImageToFIFO(streamProcess.fifoFile, imagePath, streamID); err != nil {
		log.Printf("Error writing image to FIFO: %v", err)
	}
	// fifoFile, err := os.OpenFile(fifoPath, os.O_WRONLY, os.ModeNamedPipe)
	// if err != nil {
	// 	log.Printf("Error opening FIFO for writing: %v", err)
	// 	return
	// }
	// defer fifoFile.Close()

	// imgFile, err := os.Open(imagePath)
	// if err != nil {
	// 	log.Printf("Error opening image file %s: %v", imagePath, err)
	// 	return
	// }
	// defer imgFile.Close()

	// log.Printf("Writing image %s to FIFO %s", imagePath, fifoPath)
	// if _, err := io.Copy(fifoFile, imgFile); err != nil {
	// 	log.Printf("Error writing image to FIFO: %v", err)
	// } else {
	// 	log.Printf("Successfully wrote image to FIFO for stream %s", streamID)
	// }
}

func (s *Streamer) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for streamPath, process := range s.activeStreams {
		log.Printf("Shutting down stream: %s", streamPath)
		// Close the FIFO file to signal FFmpeg to terminate
		if err := process.fifoFile.Close(); err != nil {
			log.Printf("Error closing FIFO for %s: %v", streamPath, err)
		}
		// Send interrupt to FFmpeg
		if err := process.cmd.Process.Signal(os.Interrupt); err != nil {
			log.Printf("Error sending interrupt to FFmpeg for %s: %v", streamPath, err)
		}
	}
}
