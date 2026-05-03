// trim-screenshot trims trailing background-colored rows from a PNG.
//
// Used by scripts/screenshots-dashboard.sh after chromium captures a
// screenshot at a generous window size. The screenshot has the actual
// dashboard content at top and a tail of dark background below; this
// program detects the bottom edge of content and crops the PNG to that
// height. Width is preserved.
//
// Background detection: probe the bottom-left pixel (assumed empty
// background). A row is considered "content" if any pixel in it differs
// from the background color by more than `tolerance` per RGB channel.
//
// Usage:
//
//	go run scripts/trim-screenshot.go path/to/image.png
//
// Modifies the file in place. Exit code 0 on success or no-op (image
// already tight); non-zero on error.
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

// tolerance is the per-channel RGB delta below which two pixels are
// considered the same color. Image RGB values are 16-bit (0..65535), so
// a tolerance of 16*256 ≈ "1 step in 8-bit".
const tolerance = 16 * 256

// bottomPadding is the number of background-colored rows kept after the
// last content row. Gives a small visual breathing room below tiles in
// the README rendering.
const bottomPadding = 20

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: trim-screenshot <png>")
		os.Exit(2)
	}
	path := os.Args[1]

	img, err := decode(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode %s: %v\n", path, err)
		os.Exit(1)
	}

	bounds := img.Bounds()
	w := bounds.Max.X - bounds.Min.X
	h := bounds.Max.Y - bounds.Min.Y

	// Background sample: bottom-left corner.
	bgR, bgG, bgB, _ := img.At(bounds.Min.X, bounds.Max.Y-1).RGBA()

	// Find lowest content row (scan from bottom up).
	contentBottom := bounds.Max.Y
	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		hasContent := false
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if absDelta(r, bgR) > tolerance ||
				absDelta(g, bgG) > tolerance ||
				absDelta(b, bgB) > tolerance {
				hasContent = true
				break
			}
		}
		if hasContent {
			contentBottom = y + 1 + bottomPadding
			if contentBottom > bounds.Max.Y {
				contentBottom = bounds.Max.Y
			}
			break
		}
	}

	newH := contentBottom - bounds.Min.Y
	if newH >= h {
		// Already tight; no trim needed.
		fmt.Printf("trim-screenshot: %s already tight (%dx%d)\n", path, w, h)
		return
	}

	// Crop into a new RGBA image to ensure deterministic encoding.
	cropped := image.NewRGBA(image.Rect(0, 0, w, newH))
	for y := bounds.Min.Y; y < contentBottom; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cropped.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}

	if err := encode(path, cropped); err != nil {
		fmt.Fprintf(os.Stderr, "encode %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("trim-screenshot: %s %dx%d -> %dx%d\n", path, w, h, w, newH)
}

func decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func encode(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func absDelta(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
