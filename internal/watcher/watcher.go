package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	path    string
	watcher *fsnotify.Watcher
}

func New(path string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		path:    path,
		watcher: fsWatcher,
	}, nil
}

func (w *Watcher) Start(ctx context.Context, jobs chan<- WatcherJob) {
	defer w.watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case <-ctx.Done():
				done <- true
				return
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
					fi, err := os.Stat(event.Name)
					if err != nil {
						log.Printf("Error stating file: %v", err)
						continue
					}
					if fi.IsDir() {
						log.Printf("Directory event detected, ignoring: %s", event.Name)
						continue // Ignore directory events
					}

					if !isImageFile(event.Name) {
						log.Printf("Non-image file detected, ignoring: %s", event.Name)
						continue // Ignore non-image files
					}

					log.Println("File created or modified:", event.Name)
					streamID := filepath.Base(filepath.Dir(event.Name))
					select {
					case jobs <- WatcherJob{FilePath: event.Name, StreamID: streamID}:
					case <-ctx.Done():
						return
					}
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				log.Println("Error:", err)
			}
		}
	}()

	err := w.watcher.Add(w.path)
	if err != nil {
		log.Fatal(err)
	}

	<-done
}

// isImageFile checks if the file has a valid image extension
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".bmp", ".gif":
		return true
	default:
		return false
	}
}

type WatcherJob struct {
	FilePath string
	StreamID string
}
