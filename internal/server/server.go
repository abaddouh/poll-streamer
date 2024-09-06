package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Server struct {
	port       int
	outputPath string
	srv        *http.Server
}

func New(port int, outputPath string) *Server {
	return &Server{
		port:       port,
		outputPath: outputPath,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream/", s.streamHandler)
	mux.HandleFunc("/shutdown", s.shutdownHandler)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	log.Printf("Starting HTTP server on port %d...\n", s.port)
	log.Printf("HLS stream should be available at: http://localhost:%d/stream/stream.m3u8\n", s.port)

	go func() {
		<-ctx.Done()
		log.Println("Server is shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) streamHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request for: %s", r.URL.Path)

	filePath := filepath.Join(s.outputPath, r.URL.Path[len("/stream/"):])
	log.Printf("Attempting to serve file: %s", filePath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("File does not exist: %s", filePath)
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filePath)
}

func (s *Server) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Server is shutting down..."))

	go func() {
		time.Sleep(100 * time.Millisecond)
		if err := s.srv.Shutdown(context.Background()); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()
}
