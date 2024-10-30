package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"strings"

	archivewalk "github.com/crimro-se/imagedb/archivewalk"
	"github.com/crimro-se/imagedb/imageutil"
	"golang.org/x/image/webp"
)

func main() {
	ctx := context.Background()
	errCh := make(chan error, 5)
	defer close(errCh)
	go func() {
		for err := range errCh {
			fmt.Println(err)
		}
	}()

	aw := archivewalk.NewArchiveWalker(10, errCh, true, true, imagehandler)
	aw.Walk("test_data/valid", ctx)
}

// 1. reads pre-processed image data from a channel,
// 2. obtains embedding,
// 3. updates database
func embeddingWorker() {}

// callback function for archivewalk,
// loads and resizes images, sends data to a channel for further processing afterwards.
func imagehandler(path, vpath string, file io.Reader, d fs.DirEntry) error {
	ext := ""
	if len(vpath) > 0 {
		ext = getExt(vpath)
	} else {
		ext = getExt(path)
	}
	var err error

	// TODO: db checks based on this (info.ModTime)
	// might need another channel for requesting and checking this data.
	info, err := d.Info()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	_ = info.ModTime()

	var img image.Image
	switch ext {
	case "jpg":
		img, err = jpeg.Decode(file)
	case "png":
		img, err = png.Decode(file)
	case "webp":
		img, err = webp.Decode(file)
	default:
		return nil
	}
	if err != nil {
		fmt.Println(err)
	}
	imageutil.ScaleImagePaddedSquareRGBA(img, color.RGBA{255, 255, 255, 255}, 256)

	return nil
}

// returns the file extension in lower-case.
// todo: special case for .tar.xz etc maybe.
func getExt(path string) string {
	splitName := strings.Split(path, ".")
	ext := strings.ToLower(splitName[len(splitName)-1])
	return ext
}
