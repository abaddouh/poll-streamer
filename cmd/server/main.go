package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
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

	flag.Parse()

	if *imagePath == "" {
		log.Fatal("Please provide the path to the image directory using the -path flag or IMAGE_PATH environment variable")
	}

	if *outputPath == "" {
		*outputPath = "./stream"
	}

	w, err := watcher.New(*imagePath)
	if err != nil {
		log.Fatal(err)
	}

	s := streamer.New(*outputPath, *frameRate, *resolution, *bitrate)

	srv := server.New(*port, *outputPath)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start the worker pool
	jobQueue := make(chan string, 100)
	for i := 0; i < *workerCount; i++ {
		wg.Add(1)
		go worker(ctx, &wg, s, jobQueue)
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

func worker(ctx context.Context, wg *sync.WaitGroup, s *streamer.Streamer, jobs <-chan string) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case imagePath, ok := <-jobs:
			if !ok {
				return
			}
			s.ProcessImage(imagePath)
		}
	}
}
