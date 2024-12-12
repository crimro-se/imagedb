package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	"github.com/crimro-se/imagedb/embeddingserver"
	"github.com/crimro-se/imagedb/imageutil"
	"github.com/crimro-se/imagedb/safeperiodicchecker"
	"github.com/crimro-se/imagedb/threadboundresourcepool"
	"golang.org/x/image/webp"
)

const MAXIMAGESIZE = 512
const MAXQUEUESIZE = 96
const COOLDOWN = 300 * time.Millisecond
const APISERVER = "http://localhost:5000"

// handles digesting images into the database &  embedding server
type ImageProcessor struct {
	dbConnections         *threadboundresourcepool.ThreadResource[*Database] // per-thread db connection pool
	basedir_id            int                                                // foreign key to use for all images we add to the db
	embeddingQueueChecker *safeperiodicchecker.Checker[int]                  // we track the servers queue size to pace our requests
	apiServer             *embeddingserver.Client
}

func NewImageProcessor(dbfile string, basedir_id int) (*ImageProcessor, error) {
	if len(dbfile) < 1 {
		return nil, fmt.Errorf("database filename can't be empty")
	}
	processor := ImageProcessor{
		basedir_id: basedir_id,
		dbConnections: threadboundresourcepool.New(
			// a function that can create new db connections
			func() *Database {
				db, err := NewDatabase(dbfile, false)
				// TODO: send a CRITICAL err to GUI
				if err != nil {
					panic(err)
				}
				return db
			}),
		apiServer: embeddingserver.New(APISERVER),
	}
	processor.embeddingQueueChecker = safeperiodicchecker.New(
		// function to return the current queue size
		func() int {
			q, err := processor.apiServer.GetQueueSize()
			if err != nil {
				panic(err) // TODO: send ciritcal err to GUI
			}
			return q
		}, COOLDOWN)
	return &processor, nil
}

// translate archive walker path division into one compatible with the database.
func (p *ImageProcessor) archiveWalkerPathToDatabasePath(path, vpath string) (string, string) {
	NoVpath := len(vpath) == 0
	var dir, file string
	if NoVpath {
		dir, file = filepath.Split(path)
		dir = filepath.Clean(dir)
		return dir, file
	}

	return filepath.Clean(path), vpath

}

// This is a callback function for archivewalk,
// loads and resizes images, sends data to a channel for further processing afterwards.
func (p *ImageProcessor) Handler(path, vpath string, file io.Reader, d fs.DirEntry, threadID int) error {
	var ext string
	if len(vpath) > 0 {
		ext = getExt(vpath)
	} else {
		ext = getExt(path)
	}

	db := p.dbConnections.GetResource(threadID)

	//todo: support all image formats trivially possible.
	var img image.Image
	var err error
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
		return fmt.Errorf("error while loading image file: %s:%s: %w", path, vpath, err)
	}

	//incomplete
	dbImg := Image{
		Width:     int64(img.Bounds().Dx()),
		Height:    int64(img.Bounds().Dy()),
		BasedirID: int64(p.basedir_id),
	}
	parentDir, fileName := p.archiveWalkerPathToDatabasePath(path, vpath)
	dbImg.Path = parentDir
	dbImg.SubPath = fileName
	id, err := db.CreateUpdateImage(&dbImg)
	if err != nil {
		return fmt.Errorf("error adding image to database: %s:%s: %w", path, vpath, err)
	}
	dbImg.ID = id
	//imageutil.ScaleImagePaddedSquareRGBA(img, color.RGBA{255, 255, 255, 255}, 256)
	if max(dbImg.Width, dbImg.Height) > MAXIMAGESIZE {
		img = imageutil.ScaleImageRGBA(img, MAXIMAGESIZE)
	}
	imgBytes, err := imageutil.ImageToPNG(img)
	if err != nil {
		return fmt.Errorf("error converting image to png: %s:%s: %w", path, vpath, err)
	}
	img = nil

	// stall & sleep if the queue is large
	for queueState := p.embeddingQueueChecker.Call(); queueState > MAXQUEUESIZE; {
		time.Sleep(COOLDOWN)
	}
	err = p.apiServer.SubmitImageTask(strconv.FormatInt(id, 10), imgBytes)

	return err
}
