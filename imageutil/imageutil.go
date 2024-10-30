// my own image convenience functions
package imageutil

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/crimro-se/imagedb/stbresize"
)

// scales an image whilst preserving aspect ratio
// this will incur an allocation for the result, and an *extra* allocation if input image isn't RGBA.
func ScaleImageRGBA(img image.Image, maxSize int) *image.RGBA {
	var img_rgba *image.RGBA
	img_rgba, ok := img.(*image.RGBA)
	if !ok {
		img_rgba = image.NewRGBA(img.Bounds())
		draw.Draw(img_rgba, img_rgba.Bounds(), img, image.Point{}, draw.Src)
	}

	// 2. resize
	newsize := CalculateNewSize(img_rgba.Bounds(), maxSize)
	img3 := image.NewRGBA(newsize)
	stbresize.StbirResizeUint8LinearRGBA(img_rgba, img3, newsize)
	return img3
}

// scales an immage to a centered padded square of specific size.
func ScaleImagePaddedSquareRGBA(img image.Image, pad color.RGBA, size int) *image.RGBA {
	var img_rgba *image.RGBA
	img_rgba, ok := img.(*image.RGBA)
	if !ok {
		img_rgba = image.NewRGBA(img.Bounds())
		draw.Draw(img_rgba, img_rgba.Bounds(), img, image.Point{}, draw.Src)
	}
	newSize := CalculateNewSize(img_rgba.Bounds(), size)
	img3 := image.NewRGBA(newSize)
	stbresize.StbirResizeUint8LinearRGBA(img_rgba, img3, newSize)

	outputSize := image.Rect(0, 0, size, size)
	output := image.NewRGBA(outputSize)
	draw.Draw(output, outputSize, &image.Uniform{pad}, image.Point{}, draw.Src)
	offset := minDiff(newSize, outputSize) / 2

	if newSize.Dx() > newSize.Dy() {
		newSize = newSize.Add(image.Pt(0, offset))
		draw.Draw(output, newSize, img3, image.Pt(0, 0), draw.Src)
	} else {
		newSize = newSize.Add(image.Pt(offset, 0))
		draw.Draw(output, newSize, img3, image.Pt(0, 0), draw.Src)
	}
	return output
}

// the difference between the size of the samller sides of input rects.
func minDiff(a, b image.Rectangle) int {
	min_a := min(a.Dx(), a.Dy())
	min_b := min(b.Dy(), b.Dx())
	return int(math.Abs(float64(min_a - min_b)))
}

// scales rect imgB until its maximum dimension matches maxDim
func CalculateNewSize(imgB image.Rectangle, maxDim int) image.Rectangle {
	minPt := image.Point{X: 0, Y: 0}
	maxSide := max(imgB.Dx(), imgB.Dy())
	scale := float64(maxDim) / float64(maxSide)
	maxPt := image.Point{
		X: int(scale * float64(imgB.Dx())),
		Y: int(scale * float64(imgB.Dy())),
	}
	return image.Rectangle{
		Min: minPt,
		Max: maxPt,
	}
}
