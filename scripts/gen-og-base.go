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

func main() {
	generateBase()
	generateHome()
}

func generateBase() {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))

	cream := color.RGBA{0xFA, 0xFA, 0xF7, 0xFF}
	lavender := color.RGBA{0xDD, 0xD6, 0xF3, 0xFF}
	green := color.RGBA{0x8F, 0xD5, 0xA6, 0xFF}

	// Fill cream background
	for y := 0; y < 630; y++ {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, cream)
		}
	}

	// Left lavender bar (8px wide)
	for y := 0; y < 630; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, lavender)
		}
	}

	// Bottom green strip (6px tall)
	for y := 624; y < 630; y++ {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, green)
		}
	}

	writeImage(img, "site/assets/og-base.png")
}

func generateHome() {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))

	dark := color.RGBA{0x1A, 0x1A, 0x1A, 0xFF}
	lavender := color.RGBA{0xDD, 0xD6, 0xF3, 0x60}
	green := color.RGBA{0x8F, 0xD5, 0xA6, 0x50}
	yellow := color.RGBA{0xF5, 0xD5, 0x6A, 0x40}

	// Fill dark background
	for y := 0; y < 630; y++ {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, dark)
		}
	}

	// Decorative circles (subtle, semi-transparent)
	drawCircle(img, 1080, 90, 70, lavender)
	drawCircle(img, 120, 540, 60, green)
	drawCircle(img, 1020, 420, 25, yellow)

	writeImage(img, "site/assets/og-home.png")
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if x < 0 || x >= 1200 || y < 0 || y >= 630 {
				continue
			}
			dx := float64(x - cx)
			dy := float64(y - cy)
			if math.Sqrt(dx*dx+dy*dy) <= float64(r) {
				img.Set(x, y, c)
			}
		}
	}
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
