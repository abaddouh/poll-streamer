package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func main() {
	width := flag.Int("width", 640, "Width of the placeholder image")
	height := flag.Int("height", 480, "Height of the placeholder image")
	outputPath := flag.String("output", "placeholder.jpg", "Output path for the placeholder image")
	text := flag.String("text", "Placeholder Image", "Text to display on the placeholder image")
	flag.Parse()

	img := image.NewRGBA(image.Rect(0, 0, *width, *height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{200, 200, 200, 255}}, image.Point{}, draw.Src)

	addLabel(img, *width/2-60, *height/2, *text)

	f, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("Error creating placeholder image: %v", err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, nil); err != nil {
		log.Fatalf("Error encoding placeholder image: %v", err)
	}

	log.Printf("Placeholder image created: %s", *outputPath)
}

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
