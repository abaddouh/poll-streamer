package streamer

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
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
	FIFOPath string
	stopChan chan struct{}
}

// type Stream struct {
// 	// ... existing fields ...
// 	FIFOPath string
// }

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

func (s *Streamer) startPersistentFFmpeg(fifoPath string, streamID string) (*exec.Cmd, *os.File, error) {
	streamPath := filepath.Join(s.outputPath, "stream", streamID)
	cmd := exec.Command("ffmpeg",
		"-y",
		"-re",
		"-f", "image2pipe",
		"-framerate", fmt.Sprintf("%d", s.frameRate),
		"-i", fifoPath,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-vf", fmt.Sprintf("fps=%d", s.frameRate),
		"-g", fmt.Sprintf("%d", s.frameRate*2),
		"-pix_fmt", "yuv420p",
		"-s", s.resolution,
		"-b:v", s.bitrate,
		"-maxrate", s.bitrate,
		"-bufsize", s.bitrate,
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+append_list",
		"-hls_segment_filename", filepath.Join(streamPath, "segment%03d.ts"),
		filepath.Join(streamPath, "stream.m3u8"),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start FFmpeg: %v", err)
	}

	// // Log FFmpeg output in real-time
	// go func() {
	// 	scanner := bufio.NewScanner(&stderr)
	// 	for scanner.Scan() {
	// 		log.Printf("FFmpeg [%s]: %s", streamPath, scanner.Text())
	// 	}
	// }()

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
			log.Printf("FFmpeg stdout: %s", stdout.String())
			log.Printf("FFmpeg stderr: %s", stderr.String())
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

func (s *Streamer) ProcessImage(streamID, imagePath string) error {
	stream, exists := s.activeStreams[streamID]
	if !exists {
		return fmt.Errorf("stream %s does not exist", streamID)
	}

	// Use the existing FIFO
	fifoPath := stream.FIFOPath

	// Open the FIFO for writing
	fifo, err := os.OpenFile(fifoPath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("failed to open FIFO for writing: %v", err)
	}
	defer fifo.Close()
	streamPath := filepath.Join(s.outputPath, "stream", streamID)

	// Start FFmpeg if not already running
	if _, exists := s.activeStreams[streamID]; !exists {
		cmd, fifoFile, err := s.startPersistentFFmpeg(fifoPath, streamID)
		if err != nil {
			log.Printf("Error starting FFmpeg for %s: %v", streamPath, err)
			return fmt.Errorf("error starting FFmpeg for %s: %v", streamPath, err)
		}
		stopChan := make(chan struct{})
		s.activeStreams[streamID] = &StreamProcess{
			cmd:      cmd,
			fifoFile: fifoFile,
			stopChan: stopChan,
			FIFOPath: fifoPath,
		}
		log.Printf("Started FFmpeg for stream %s with PID %d", streamPath, cmd.Process.Pid)

		// Start keepStreamAlive in a separate goroutine
		go s.keepStreamAlive(streamPath, stopChan)
	}

	// Write the new image to the FIFO
	streamProcess := s.activeStreams[streamPath]
	if streamProcess == nil {
		log.Printf("No active stream process found for %s after initialization", streamPath)
		return fmt.Errorf("no active stream process found for %s after initialization", streamPath)
	}

	if err := s.writeImageToFIFO(streamProcess.fifoFile, imagePath, streamID); err != nil {
		log.Printf("Error writing image to FIFO for StreamID %s: %v", streamID, err)
		return fmt.Errorf("error writing image to FIFO for StreamID %s: %v", streamID, err)
	} else {
		log.Printf("Successfully wrote image to FIFO for StreamID %s", streamID)
	}
	return nil
}

func (s *Streamer) keepStreamAlive(streamPath string, stopChan <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			log.Printf("Stopping keepStreamAlive for %s", streamPath)
			return
		case <-ticker.C:
			s.mu.Lock()
			if _, exists := s.activeStreams[streamPath]; exists {
				// Check if m3u8 file exists
				m3u8Path := filepath.Join(streamPath, "stream.m3u8")
				if _, err := os.Stat(m3u8Path); os.IsNotExist(err) {
					log.Printf("M3U8 file not found for %s, waiting...", streamPath)
				} else {
					log.Printf("M3U8 file exists for %s", streamPath)
				}

				// Write placeholder image only if no new image has been processed
				// if err := s.writeImageToFIFO(process.fifoFile, s.placeholderImg, filepath.Base(streamPath)); err != nil {
				// 	log.Printf("Error writing placeholder to FIFO for %s: %v", streamPath, err)
				// }
			} else {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
		}
	}
}

func (s *Streamer) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for streamPath, process := range s.activeStreams {
		log.Printf("Shutting down stream: %s", streamPath)
		close(process.stopChan)
		if err := process.fifoFile.Close(); err != nil {
			log.Printf("Error closing FIFO for %s: %v", streamPath, err)
		}
		if err := process.cmd.Process.Signal(os.Interrupt); err != nil {
			log.Printf("Error sending interrupt to FFmpeg for %s: %v", streamPath, err)
		}
	}
}
