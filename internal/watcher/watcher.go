package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type WatcherJob struct {
	FilePath string
	StreamID string
}

type Watcher struct {
	watcher   *fsnotify.Watcher
	imagePath string
}

func New(imagePath string) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %v", err)
	}

	// Add the imagePath to the watcher
	err = fw.Add(imagePath)
	if err != nil {
		fw.Close()
		return nil, fmt.Errorf("failed to add path to watcher: %v", err)
	}

	return &Watcher{
		watcher:   fw,
		imagePath: imagePath,
	}, nil
}

// isImageFile checks if the given filename has a valid image extension.
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	valid := false
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff":
		valid = true
	}
	log.Printf("isImageFile: %s -> %v", filename, valid)
	return valid
}

func (w *Watcher) Start(ctx context.Context, jobs chan<- WatcherJob) {
	var (
		eventDebounce  = 100 * time.Millisecond
		lastEventTimes = make(map[string]time.Time)
		debounceMutex  sync.Mutex
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Watcher received shutdown signal")
				w.watcher.Close()
				return
			case event, ok := <-w.watcher.Events:
				if !ok {
					log.Println("Watcher events channel closed")
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
					fi, err := os.Stat(event.Name)
					if err != nil {
						log.Printf("Error stating file: %v", err)
						continue
					}
					if fi.IsDir() {
						log.Printf("New directory detected: %s", event.Name)
						// Add the new directory to the watcher
						if err := w.watcher.Add(event.Name); err != nil {
							log.Printf("Error adding new directory to watcher: %v", err)
						} else {
							log.Printf("Now watching new directory: %s", event.Name)
						}
						continue
					}

					if !isImageFile(event.Name) {
						log.Printf("Non-image file detected, ignoring: %s", event.Name)
						continue
					}

					// Debounce logic
					debounceMutex.Lock()
					lastTime, exists := lastEventTimes[event.Name]
					now := time.Now()
					if exists && now.Sub(lastTime) < eventDebounce {
						debounceMutex.Unlock()
						continue // Skip this event as it's too soon after the last one
					}
					lastEventTimes[event.Name] = now
					debounceMutex.Unlock()

					log.Println("File created or modified:", event.Name)
					streamID := filepath.Base(filepath.Dir(event.Name))
					log.Printf("Extracted StreamID: %s from FilePath: %s", streamID, event.Name)

					select {
					case jobs <- WatcherJob{FilePath: event.Name, StreamID: streamID}:
						log.Printf("Job enqueued: StreamID=%s, FilePath=%s", streamID, event.Name)
					case <-ctx.Done():
						return
					}
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					log.Println("Watcher errors channel closed")
					return
				}
				log.Println("Watcher Error:", err)
			}
		}
	}()
}
