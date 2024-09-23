package watcher

import (
	"context"
	"log"
	"path/filepath"

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

type WatcherJob struct {
	FilePath string
	StreamID string
}
