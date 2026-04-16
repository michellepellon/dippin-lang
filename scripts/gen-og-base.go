// ABOUTME: Generates base OG image PNGs for Hugo text overlay.
// ABOUTME: Run once to create site/assets/og-base.png and site/assets/og-home.png.
//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const imgW, imgH = 1200, 630

func main() {
	generateBase()
	generateHome()
}

func generateBase() {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	cream := color.RGBA{0xFA, 0xFA, 0xF7, 0xFF}
	lavender := color.RGBA{0xDD, 0xD6, 0xF3, 0xFF}
	green := color.RGBA{0x8F, 0xD5, 0xA6, 0xFF}

	fillRect(img, 0, 0, imgW, imgH, cream)
	fillRect(img, 0, 0, 8, imgH, lavender)
	fillRect(img, 0, 624, imgW, 6, green)

	writeImage(img, "site/assets/og-base.png")
}

func generateHome() {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	dark := color.RGBA{0x1A, 0x1A, 0x1A, 0xFF}
	lavender := color.RGBA{0xDD, 0xD6, 0xF3, 0x60}
	green := color.RGBA{0x8F, 0xD5, 0xA6, 0x50}
	yellow := color.RGBA{0xF5, 0xD5, 0x6A, 0x40}

	fillRect(img, 0, 0, imgW, imgH, dark)
	drawCircle(img, 1080, 90, 70, lavender)
	drawCircle(img, 120, 540, 60, green)
	drawCircle(img, 1020, 420, 25, yellow)

	writeImage(img, "site/assets/og-home.png")
}

func fillRect(img *image.RGBA, x0, y0, w, h int, c color.RGBA) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			img.Set(x, y, c)
		}
	}
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if inBounds(x, y) && inRadius(x, y, cx, cy, r) {
				img.Set(x, y, c)
			}
		}
	}
}

func inBounds(x, y int) bool {
	return x >= 0 && x < imgW && y >= 0 && y < imgH
}

func inRadius(x, y, cx, cy, r int) bool {
	dx := float64(x - cx)
	dy := float64(y - cy)
	return math.Sqrt(dx*dx+dy*dy) <= float64(r)
}

func writeImage(img *image.RGBA, path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
