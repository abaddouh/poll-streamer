#!/bin/bash

## Requirements

# - ffmpeg
# - vlc (command line)
# - jq
# - python3
# - python3 dependencies in requirements.txt

# ==============================================================================
# run_demo.sh
#
# Automated script to demonstrate Poll Streamer.
# Allows user to choose between VLC and Web Browser for viewing the stream.
# Ensures all processes are cleanly terminated upon exit.
# ==============================================================================

# Exit immediately if a command exits with a non-zero status
set -e

# Function to display the menu
show_menu() {
    echo ""
    echo "Select an option to view the stream:"
    echo "1) Open VLC Media Player"
    echo "2) Open Web Browser"
    echo "3) Exit Demo"
    echo -n "Enter your choice [1-3]: "
}

# Function to cleanup background processes
cleanup() {
    echo ""
    echo "Cleaning up..."
    if [ -n "$SERVER_PID" ]; then
        echo "Stopping Poll Streamer server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
    fi
    if [ -n "$IMAGE_GEN_PID" ]; then
        echo "Stopping Image Generator (PID: $IMAGE_GEN_PID)..."
        kill $IMAGE_GEN_PID 2>/dev/null || true
    fi
    if [ -n "$HTML_SERVER_PID" ]; then
        echo "Stopping HTML Server (PID: $HTML_SERVER_PID)..."
        kill $HTML_SERVER_PID 2>/dev/null || true
    fi
    if [ -n "$VLC_PID" ]; then
        echo "Closing VLC (PID: $VLC_PID)..."
        kill $VLC_PID 2>/dev/null || true
    fi
    echo "All processes terminated."
    exit 0
}

# Trap SIGINT and SIGTERM to cleanup
trap cleanup SIGINT SIGTERM

# ==============================================================================
# Preparation: Ensure placeholder image exists
# ==============================================================================

echo "Generating placeholder image..."
go run cmd/placeholder/main.go
echo "Placeholder image generated."

# ==============================================================================
# Start Poll Streamer Server
# ==============================================================================

IMAGE_DIR="./images"
STREAM_DIR="./stream"

echo "Starting Poll Streamer Server..."
go run cmd/server/main.go \
  -path "$IMAGE_DIR" \
  -output "$STREAM_DIR" \
  -fps 30 \
  -resolution 1280x720 \
  -bitrate 1000k \
  -port 8080 \
  -workers 4 \
  -placeholder ./placeholder.jpg &
SERVER_PID=$!
echo "Poll Streamer Server started with PID: $SERVER_PID"

# Allow server to initialize
sleep 2

# ==============================================================================
# Generate a New Stream
# ==============================================================================

echo "Generating a new stream..."
RESPONSE=$(curl -s -X POST http://localhost:8080/generate-stream)
STREAM_ID=$(echo "$RESPONSE" | jq -r '.stream_id')
STREAM_URL=$(echo "$RESPONSE" | jq -r '.stream_url')

echo "Stream ID: $STREAM_ID"
echo "Stream URL: $STREAM_URL"

# ==============================================================================
# Start Generating Images
# ==============================================================================
# Run indefinitely without outputting to stdout or stderr

echo "Starting image generation indefinitely..."
python test/generate_images.py \
  --output "$IMAGE_DIR/$STREAM_ID" \
  --interval 1 \
  > /dev/null 2>&1 &
IMAGE_GEN_PID=$!
echo "Image Generator started with PID: $IMAGE_GEN_PID"

# ==============================================================================
# Update HTML Player with Stream URL
# ==============================================================================

echo "Updating HTML player with the stream URL..."
HTML_PLAYER_FILE="test/video_player.html"

# Check if the sed command is compatible (macOS vs Linux)
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' "s|var videoSrc = '.*';|var videoSrc = '$STREAM_URL';|" "$HTML_PLAYER_FILE"
else
    # Linux and others
    sed -i "s|var videoSrc = '.*';|var videoSrc = '$STREAM_URL';|" "$HTML_PLAYER_FILE"
fi

echo "HTML player updated."

# ==============================================================================
# Start HTTP Server for HTML Player
# ==============================================================================

echo "Starting HTTP server for HTML player on port 4000..."
cd test
python -m http.server 4000 &
HTML_SERVER_PID=$!
cd ..
echo "HTML Server started with PID: $HTML_SERVER_PID"

# ==============================================================================
# Function to Open VLC
# ==============================================================================

open_vlc() {
    echo "Opening VLC Media Player..."
    vlc "$STREAM_URL" &
    VLC_PID=$!
    echo "VLC opened with PID: $VLC_PID"
}

# ==============================================================================
# Function to Open Web Browser
# ==============================================================================

open_browser() {
    echo "Opening Web Browser to view the stream..."
    if command -v xdg-open > /dev/null; then
        # Linux
        xdg-open "http://localhost:4000/video_player.html"
    elif command -v open > /dev/null; then
        # macOS
        open "http://localhost:4000/video_player.html"
    elif command -v start > /dev/null; then
        # Windows (via Git Bash or similar)
        start "http://localhost:4000/video_player.html"
    else
        echo "Please open your web browser and navigate to http://localhost:4000/video_player.html"
    fi
}

# ==============================================================================
# Interactive Menu
# ==============================================================================

while true; do
    show_menu
    read -r choice
    case $choice in
        1)
            open_vlc
            ;;
        2)
            open_browser
            ;;
        3)
            cleanup
            ;;
        *)
            echo "Invalid choice. Please enter 1, 2, or 3."
            ;;
    esac
done
