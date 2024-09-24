package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"github.com/abaddouh/poll-streamer/internal/server"
	"github.com/abaddouh/poll-streamer/internal/streamer"
	"github.com/abaddouh/poll-streamer/internal/watcher"
)

func main() {
	imagePath := flag.String("path", os.Getenv("IMAGE_PATH"), "Path to the directory containing images")
	outputPath := flag.String("output", os.Getenv("OUTPUT_PATH"), "Path to output the HLS stream files")
	frameRate := flag.Int("fps", 30, "Frames per second for the output video")
	resolution := flag.String("resolution", "640x480", "Resolution of the output video")
	bitrate := flag.String("bitrate", "500k", "Bitrate of the output video")
	port := flag.Int("port", 8080, "Port to serve the HLS stream")
	workerCount := flag.Int("workers", runtime.NumCPU(), "Number of worker goroutines")
	placeholderImg := flag.String("placeholder", "./placeholder.jpg", "Path to the placeholder image")

	flag.Parse()

	// Validate and create paths if necessary
	if err := ensureDir(*imagePath); err != nil {
		log.Fatalf("Error with image path: %v", err)
	}

	if *outputPath == "" {
		*outputPath = "./stream"
	}

	if err := ensureDir(*outputPath); err != nil {
		log.Fatalf("Error with output path: %v", err)
	}

	placeholderDir := filepath.Dir(*placeholderImg)
	if err := ensureDir(placeholderDir); err != nil {
		log.Fatalf("Error with placeholder image directory: %v", err)
	}

	if *imagePath == "" {
		log.Fatal("Please provide the path to the image directory using the -path flag or IMAGE_PATH environment variable")
	}

	w, err := watcher.New(*imagePath)
	if err != nil {
		log.Fatal(err)
	}

	s := streamer.New(*outputPath, *frameRate, *resolution, *bitrate, *placeholderImg)

	srv := server.New(*port, *outputPath, *placeholderImg)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start the worker pool
	jobQueue := make(chan watcher.WatcherJob, 100)
	for i := 0; i < *workerCount; i++ {
		wg.Add(1)
		go worker(ctx, &wg, s, srv, jobQueue)
	}

	// Start the watcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Start(ctx, jobQueue)
	}()

	// Start the server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Start(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutting down...")
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()

	// Clean up the stream folder
	if err := os.RemoveAll(*outputPath); err != nil {
		log.Printf("Error cleaning up stream folder: %v", err)
	}

	log.Println("Shutdown complete")
}

// ensureDir checks if a directory exists, and creates it if it doesn't.
func ensureDir(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.Printf("Directory does not exist. Creating: %s", path)
		return os.MkdirAll(path, 0755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory: %s", path)
	}
	return nil
}

func worker(ctx context.Context, wg *sync.WaitGroup, s *streamer.Streamer, srv *server.Server, jobs <-chan watcher.WatcherJob) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}
			if streamPath, exists := srv.GetStreamPath(job.StreamID); exists {
				s.ProcessImage(job.FilePath, streamPath)
			} else {
				log.Printf("Stream %s not found", job.StreamID)
			}
		}
	}
}
