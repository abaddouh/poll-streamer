package server

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"strconv"

	"github.com/abaddouh/poll-streamer/internal/streamer"
	"github.com/google/uuid"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type Server struct {
	port           int
	outputPath     string
	placeholderImg string
	srv            *http.Server
	streams        map[string]string
	mu             sync.RWMutex
	streamer       *streamer.Streamer
}

// New initializes a new Server instance with a Streamer
func New(port int, outputPath, placeholderImg string, streamerInstance *streamer.Streamer) *Server {
	return &Server{
		port:           port,
		outputPath:     outputPath,
		placeholderImg: placeholderImg,
		streams:        make(map[string]string),
		streamer:       streamerInstance, // Initialize the Streamer field
	}
}

// Start begins the HTTP server and handles graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream/", s.streamHandler)
	mux.HandleFunc("/shutdown", s.shutdownHandler)
	mux.HandleFunc("/generate-stream", s.generateStreamHandler)
	mux.HandleFunc("/heartbeat", s.heartbeatHandler)
	mux.HandleFunc("/", s.homeHandler)
	mux.HandleFunc("/placeholder", s.placeholderHandler)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	log.Printf("Starting HTTP server on port %d...\n", s.port)

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

// heartbeatHandler responds with a 200 status to indicate the server is alive.
func (s *Server) heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// homeHandler provides usage instructions for the API.
func (s *Server) homeHandler(w http.ResponseWriter, r *http.Request) {
	message := `Welcome to the Poll Streamer API!

Available Routes:
- GET /heartbeat: Check if the server is running.
- POST /generate-stream: Generate a new stream.
- GET /stream/{stream_id}/stream.m3u8: Access a specific stream.
- GET /placeholder: Retrieve the current placeholder image.
- POST /placeholder: Generate a new placeholder image.

For more details on each endpoint, refer to the documentation.`

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(message))
}

// placeholderHandler manages GET and POST requests for the placeholder image.
func (s *Server) placeholderHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getPlaceholder(w, r)
	case http.MethodPost:
		s.createPlaceholder(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getPlaceholder serves the current placeholder image.
func (s *Server) getPlaceholder(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(s.placeholderImg); os.IsNotExist(err) {
		http.Error(w, "Placeholder image not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, s.placeholderImg)
}

// createPlaceholder generates a new placeholder image based on provided parameters.
func (s *Server) createPlaceholder(w http.ResponseWriter, r *http.Request) {
	// Parse parameters from query or JSON body
	width, height, text, err := parsePlaceholderParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = generatePlaceholderImage(s.placeholderImg, width, height, text)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create placeholder: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Placeholder image created successfully"))
}

// parsePlaceholderParams extracts parameters for placeholder generation.
func parsePlaceholderParams(r *http.Request) (int, int, string, error) {
	var params struct {
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Text   string `json:"text"`
	}

	// Try to parse JSON body
	if r.Header.Get("Content-Type") == "application/json" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return 0, 0, "", fmt.Errorf("invalid request body")
		}
		defer r.Body.Close()

		if err := json.Unmarshal(body, &params); err != nil {
			return 0, 0, "", fmt.Errorf("invalid JSON format")
		}
	} else {
		// Fallback to query parameters
		query := r.URL.Query()
		params.Width = atoiDefault(query.Get("width"), 640)
		params.Height = atoiDefault(query.Get("height"), 480)
		params.Text = query.Get("text")
	}

	if params.Width <= 0 || params.Height <= 0 {
		return 0, 0, "", fmt.Errorf("width and height must be positive integers")
	}

	if params.Text == "" {
		params.Text = "Placeholder Image"
	}

	return params.Width, params.Height, params.Text, nil
}

// atoiDefault converts string to int with a default value.
func atoiDefault(s string, defaultVal int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return defaultVal
}

// generatePlaceholderImage creates a placeholder JPEG image.
func generatePlaceholderImage(path string, width, height int, text string) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{200, 200, 200, 255}}, image.Point{}, draw.Src)

	addLabel(img, width/2-60, height/2, text)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating placeholder image: %v", err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, nil); err != nil {
		return fmt.Errorf("error encoding placeholder image: %v", err)
	}

	log.Printf("Placeholder image created: %s", path)
	return nil
}

// addLabel draws text on the image.
func addLabel(img *image.RGBA, x, y int, label string) {
	col := color.RGBA{50, 50, 50, 255}
	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(label)
}

// shutdownHandler gracefully shuts down the server.
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

// generateStreamHandler creates a new stream and returns its details.
func (s *Server) generateStreamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	streamID := uuid.New().String()
	streamPath := fmt.Sprintf("/stream/%s/stream.m3u8", streamID)
	fullStreamPath := filepath.Join(s.outputPath, streamID)

	s.mu.Lock()
	s.streams[streamID] = fullStreamPath
	s.mu.Unlock()

	// Initialize the placeholder stream using the injected Streamer instance
	s.streamer.ProcessImage(s.placeholderImg, streamID)

	response := map[string]string{
		"stream_url": fmt.Sprintf("http://localhost:%d%s", s.port, streamPath),
		"stream_id":  streamID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// streamHandler serves the requested stream file.
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

// GetStreamPath retrieves the path for a given stream ID.
func (s *Server) GetStreamPath(streamID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, ok := s.streams[streamID]
	return path, ok
}
