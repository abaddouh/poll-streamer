import os
import time
from PIL import Image, ImageDraw, ImageFont
import argparse

def create_image(output_dir, index):
    # Create a new image with white background
    img = Image.new('RGB', (640, 480), color=(255, 255, 255))

    # Get a drawing context
    d = ImageDraw.Draw(img)

    # Draw text on the image
    font = ImageFont.load_default()
    d.text((10, 10), f"Test Image {index}", fill=(0, 0, 0), font=font)
    d.text((10, 30), time.strftime("%Y-%m-%d %H:%M:%S"), fill=(0, 0, 0), font=font)

    # Save the image
    img.save(os.path.join(output_dir, f'image_{index:04d}.jpg'))

def main(output_dir, interval, count):
    if not os.path.exists(output_dir):
        os.makedirs(output_dir)

    if count is not None:
        for i in range(count):
            create_image(output_dir, i)
            print(f"Created image {i + 1}/{count}")
            if i < count - 1:  # Don't sleep after the last image
                time.sleep(interval)
    else:
        i = 0
        try:
            while True:
                create_image(output_dir, i)
                print(f"Created image {i + 1}")
                i += 1
                time.sleep(interval)
        except KeyboardInterrupt:
            print("\nImage generation stopped by user.")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Generate test images for Poll Streamer')
    parser.add_argument('--output', default='./test_images', help='Output directory for images')
    parser.add_argument('--interval', type=float, default=1.0, help='Interval between image generation in seconds')
    parser.add_argument('--count', type=int, default=None, help='Number of images to generate (optional). If not provided, the script will run indefinitely.')

    args = parser.parse_args()

    main(args.output, args.interval, args.count)
