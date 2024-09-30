#!/bin/bash

FIFO="./stream/stream1/input_fifo"
IMAGE_DIR="./test_images"

# Ensure the FIFO exists
if [[ ! -p "$FIFO" ]]; then
    echo "FIFO $FIFO does not exist. Creating..."
    mkfifo "$FIFO"
fi

echo "Starting to monitor $IMAGE_DIR for new images with fswatch..."

# Open the FIFO for writing using file descriptor 3
exec 3> "$FIFO"

# Function to handle image writing
write_image() {
    local img="$1"
    echo "Detected new image: $img"

    # Wait until the file is fully written
    while lsof "$img" &> /dev/null; do
        echo "Waiting for $img to be fully written..."
        sleep 0.1
    done

    echo "Writing $img to FIFO..."
    cat "$img" >&3
    echo "Written $img to FIFO."
    rm "$img"
}

# Monitor the directory for new .jpg files using fswatch
fswatch -0 "$IMAGE_DIR" | while read -d '' event; do
    # Loop through all .jpg files in the directory
    for img in "$IMAGE_DIR"/*.jpg; do
        # Check if the file exists and is a regular file
        if [[ -f "$img" ]]; then
            write_image "$img"
        fi
    done
done

# Close the FIFO when the script exits
trap "exec 3>&-" EXIT
