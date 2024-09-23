# Start from a small Alpine Linux image with Go installed
FROM golang:1.22-alpine AS builder

# Install git and build dependencies
RUN apk add --no-cache git build-base

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source from the current directory to the working Directory inside the container
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o poll-streamer-placeholder ./cmd/placeholder

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o poll-streamer ./cmd/server

# Start a new stage from scratch
FROM alpine:latest

# Install FFmpeg
RUN apk add --no-cache ffmpeg

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/poll-streamer .
COPY --from=builder /app/poll-streamer-placeholder .
RUN 'poll-streamer-placeholder -width 1280 -height 720 -output custom_placeholder.jpg -text "Stream Coming Soon"'
# Command to run the executable
CMD ["./poll-streamer -placeholder ./custom_placeholder.jpg"]
