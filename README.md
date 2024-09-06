# Poll Streamer

Poll Streamer is a Go application that watches a directory for new images and creates a live HLS video stream from them.

## Features

- Watch a directory for new images
- Create an HLS video stream from the images
- Serve the HLS stream via HTTP
- Designed for concurrent processing and Kubernetes deployment

## Prerequisites

- Go 1.18 or higher
- FFmpeg installed and available in your system PATH
- Docker (for containerized deployment)
- Kubernetes cluster (for Kubernetes deployment)
- Python 3.x (for running the test script)

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/aimerib/poll-streamer.git
   cd poll-streamer
   ```

2. Install Go dependencies:
   ```
   go mod tidy
   ```

3. Set up Python environment for the test script:

   - On macOS and Linux:
     ```
     python3 -m venv venv
     source venv/bin/activate
     pip install -r test/requirements.txt
     ```

   - On Windows:
     ```
     python -m venv venv
     venv\Scripts\activate
     pip install -r test/requirements.txt
     ```

   Note: After you're done testing, you can deactivate the virtual environment by running:
   ```
   deactivate
   ```

## Usage

### Local Development

1. Activate the Python virtual environment:
   - On macOS and Linux:
     ```
     source venv/bin/activate
     ```
   - On Windows:
     ```
     venv\Scripts\activate
     ```

2. Start the Poll Streamer:
   ```
   go run cmd/server/main.go -path ./test_images -output ./stream -fps 30 -resolution 1280x720 -bitrate 1000k -port 8080 -workers 4
   ```

3. In a separate terminal, activate the virtual environment again and run the test script to generate sample images:
   ```
   python test/generate_images.py --output ./test_images --interval 1 --count 30
   ```

4. The HLS stream will be available at `http://localhost:8080/stream/stream.m3u8`. You can view it using a media player that supports HLS streams, such as VLC:
   ```
   vlc http://localhost:8080/stream/stream.m3u8
   ```

5. After testing, deactivate the virtual environment:
   ```
   deactivate
   ```

Options for Poll Streamer:
- `-path`: Path to the directory containing images (required)
- `-output`: Path to output the HLS stream files (default: "./stream")
- `-fps`: Frames per second for the output video (default: 30)
- `-resolution`: Resolution of the output video (default: "640x480")
- `-bitrate`: Bitrate of the output video (default: "500k")
- `-port`: Port to serve the HLS stream (default: 8080)
- `-workers`: Number of worker goroutines (default: number of CPU cores)

Options for test script:
- `--output`: Output directory for images (default: "./test_images")
- `--interval`: Interval between image generation in seconds (default: 1.0)
- `--count`: Number of images to generate (default: 10)

### Docker Deployment

1. Build the Docker image:
   ```
   docker build -t poll-streamer .
   ```

2. Run the Docker container:
   ```
   docker run -p 8080:8080 -v /path/to/images:/images -v /path/to/output:/stream -e IMAGE_PATH=/images -e OUTPUT_PATH=/stream poll-streamer
   ```

3. Generate test images as described in the Local Development section.

### Kubernetes Deployment

1. Create a Kubernetes deployment YAML file (e.g., `deployment.yaml`):

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: poll-streamer
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: poll-streamer
     template:
       metadata:
         labels:
           app: poll-streamer
       spec:
         containers:
         - name: poll-streamer
           image: your-registry/poll-streamer:latest
           env:
           - name: IMAGE_PATH
             value: "/images"
           - name: OUTPUT_PATH
             value: "/stream"
           volumeMounts:
           - name: images
             mountPath: /images
           - name: stream
             mountPath: /stream
         volumes:
         - name: images
           hostPath:
             path: /path/on/host/images
         - name: stream
           hostPath:
             path: /path/on/host/stream
   ```

2. Apply the deployment:
   ```
   kubectl apply -f deployment.yaml
   ```

3. Generate test images in the appropriate directory on the Kubernetes host.

### Shutting Down the Server
The server can be shut down gracefully in two ways:

1. By sending a SIGINT or SIGTERM signal (e.g., pressing Ctrl+C in the terminal).

2. By sending a POST request to the `/shutdown` endpoint:
   ```
   curl -X POST http://localhost:8080/shutdown
   ```

When the server shuts down, it will:
- Stop accepting new connections
- Finish processing any ongoing requests
- Clean up the stream folder (./stream by default)

## Consuming the Video Stream

### Using VLC Media Player

You can view the stream using VLC Media Player:

```
vlc http://localhost:8080/stream/stream.m3u8
```

### Using FFplay

You can also use FFplay to view the stream:

```
ffplay http://localhost:8080/stream/stream.m3u8
```

### Using a Web Browser

To view the stream in a web browser, you can use the provided HTML player:

1. Ensure Poll Streamer is running and generating the HLS stream.

2. Open the file `test/video_player.html` in a web browser.

   - If you're using a simple HTTP server to serve this file, make sure it's running on a different port than Poll Streamer.

   - For example, you can use Python's built-in HTTP server:
     ```
     python -m http.server 8000
     ```
     Then open `http://localhost:8000/test/video_player.html` in your browser.

3. The video should start playing automatically if everything is set up correctly.

Note: If you're running Poll Streamer on a different machine or port, you'll need to update the `videoSrc` variable in the `video_player.html` file accordingly.

### Embedding in Your Own Web Page

To embed the video stream in your own web page:

1. Include the hls.js library in your HTML:
   ```html
   <script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
   ```

2. Add a video element to your HTML:
   ```html
   <video id="video" controls></video>
   ```

3. Add the following JavaScript to your page:
   ```javascript
   var video = document.getElementById('video');
   var videoSrc = 'http://localhost:8080/stream/stream.m3u8';
   if (Hls.isSupported()) {
       var hls = new Hls();
       hls.loadSource(videoSrc);
       hls.attachMedia(video);
       hls.on(Hls.Events.MANIFEST_PARSED, function() {
           video.play();
       });
   }
   else if (video.canPlayType('application/vnd.apple.mpegurl')) {
       video.src = videoSrc;
       video.addEventListener('loadedmetadata', function() {
           video.play();
       });
   }
   ```

   Make sure to replace the `videoSrc` with the correct URL if you're not running Poll Streamer locally or on a different port.

This code uses hls.js if it's supported by the browser, and falls back to native HLS support for browsers like Safari that support HLS natively.

## Testing

To test the Poll Streamer:

1. Activate the Python virtual environment:
   - On macOS and Linux:
     ```
     source venv/bin/activate
     ```
   - On Windows:
     ```
     venv\Scripts\activate
     ```

2. Start the Poll Streamer as described in the Usage section.

3. In a separate terminal, activate the virtual environment again and run the test script to generate sample images:
   ```
   python test/generate_images.py --output ./test_images --interval 1 --count 30
   ```

4. Use a media player that supports HLS (like VLC) to view the stream at `http://localhost:8080/stream/stream.m3u8`.

5. You should see a video stream of the generated test images, updating every second.

6. After testing, deactivate the virtual environment:
   ```
   deactivate
   ```

If everything is working correctly, you'll see a stream of images with incrementing numbers and timestamps.

## Troubleshooting

### Common issues
- If you don't see any images in the stream, check that the Poll Streamer is watching the correct directory (specified by `-path` or `IMAGE_PATH`).
- Ensure that FFmpeg is installed and accessible in your system PATH.
- Check the Poll Streamer logs for any error messages.
- Verify that the test images are being generated in the correct directory.
- If you encounter Python-related issues:
  - Ensure you've activated the virtual environment before running the test script.
  - Try recreating the virtual environment and reinstalling the dependencies.
  - Check that your Python version is 3.x with `python --version` or `python3 --version`.

### Streaming issues
If you encounter issues with the stream not being accessible, follow these steps:

1. Check Poll Streamer logs:
   - Look for any error messages in the terminal where Poll Streamer is running.
   - Verify that the server started successfully and is listening on the correct port.

2. Verify the stream files:
   - Check that the output directory (specified by `-output` or `OUTPUT_PATH`) contains the stream files.
   - You should see files like `stream.m3u8` and several `.ts` files.
   - If these files are missing, there might be an issue with FFmpeg or file permissions.

3. Test the HTTP server:
   - Open a web browser and navigate to `http://localhost:8080/stream/stream.m3u8`
   - If you see the contents of the m3u8 file, the server is working correctly.
   - If you get a 404 error, the file might not exist or the server might be looking in the wrong directory.

4. Check FFmpeg:
   - Ensure FFmpeg is installed correctly: run `ffmpeg -version` in a terminal.
   - If FFmpeg is not recognized, add it to your system PATH.

5. Verify image generation:
   - Check that the test script is generating images in the correct directory.
   - Look for .jpg files in the directory specified by the `-path` argument to Poll Streamer.

6. Test with curl:
   - Run `curl http://localhost:8080/stream/stream.m3u8`
   - This should return the contents of the m3u8 file if the server is working correctly.

7. Firewall and antivirus:
   - Temporarily disable your firewall and antivirus to check if they're blocking the connection.

8. Try a different player:
   - If VLC doesn't work, try using FFplay: `ffplay http://localhost:8080/stream/stream.m3u8`
   - Or try opening the stream URL in a web browser that supports HLS (like Safari).

9. Check for port conflicts:
   - Ensure no other application is using port 8080.
   - Try changing the port using the `-port` option when starting Poll Streamer.

10. Permissions:
    - Ensure the user running Poll Streamer has read/write access to both the input and output directories.

### FFmpeg Issues

If you encounter FFmpeg errors, such as "exit status 234", follow these steps:

1. Check FFmpeg installation:
   ```
   ffmpeg -version
   ```
   Ensure you have a recent version of FFmpeg installed.

2. Verify input image:
   - Make sure the input image file exists and is readable.
   - Check the image format. Try with different image formats (e.g., PNG instead of JPG).

3. Check output directory:
   - Ensure the output directory exists and is writable.
   - Try with an absolute path for the output directory.

4. Run FFmpeg manually:
   Try running the FFmpeg command directly in your terminal. Replace `<input_image>` and `<output_path>` with your actual paths:
   ```
   ffmpeg -f image2 -loop 1 -i <input_image> -vf fps=30 -f hls -hls_time 2 -hls_list_size 5 -hls_flags delete_segments+append_list -codec:v libx264 -preset ultrafast -tune zerolatency -s 640x480 -b:v 500k -maxrate 500k -bufsize 500k -re -max_muxing_queue_size 1024 <output_path>/stream.m3u8
   ```
   This can help identify specific issues with the FFmpeg command.

5. Check system resources:
   - Ensure you have enough disk space.
   - Monitor CPU and memory usage while running Poll Streamer.

6. Libx264 codec:
   - Verify that your FFmpeg build includes the libx264 codec:
     ```
     ffmpeg -encoders | grep libx264
     ```
   - If it's not available, you may need to rebuild FFmpeg with libx264 support or use a different codec.

7. Simplify the command:
   If the error persists, try simplifying the FFmpeg command by removing some options. Start with a basic command and add options back one by one to identify which option is causing the issue.

8. Check FFmpeg logs:
   The updated Poll Streamer now prints FFmpeg's error output. Check the logs for more detailed error messages from FFmpeg.

If you're still experiencing issues after trying these steps, please open an issue on the GitHub repository with the following information:
- Your operating system
- FFmpeg version (`ffmpeg -version`)
- The exact error message and FFmpeg output from the Poll Streamer logs
- The contents of one of your input image files (you can use `file <image_path>` command)

## License

[MIT License](LICENSE)
