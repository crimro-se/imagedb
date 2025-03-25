package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/crimro-se/imagedb/embeddingserver"
	"github.com/crimro-se/imagedb/imageutil"
	"github.com/crimro-se/imagedb/threadboundresourcepool"
	"golang.org/x/image/webp"
)

const MAXIMAGESIZE = 512
const MAXQUEUESIZE = 96
const COOLDOWN = 250 * time.Millisecond
const APISERVER = "http://localhost:5000"

// handles digesting images into the database &  embedding server
type ImageProcessor struct {
	dbConnections *threadboundresourcepool.ThreadResource[*Database] // per-thread db connection pool
	basedir       Basedir                                            // foreign key to use for all images we add to the db
	apiServer     *embeddingserver.Client
}

func NewImageProcessor(dbfile string, basedir Basedir) (*ImageProcessor, error) {
	if len(dbfile) < 1 {
		return nil, fmt.Errorf("database filename can't be empty")
	}
	processor := ImageProcessor{
		basedir: basedir,
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
		apiServer: embeddingserver.NewClient(APISERVER),
	}
	return &processor, nil
}

// Translate archive walker path division into one compatible with the database.
// The archive walker form is a path to a file, and a virtual path for files within compressed archives.
// The database form is parent directory OR archive, and a filename/path.
// Additionally, we need to account for basedir
func (p *ImageProcessor) archiveWalkerPathToDatabasePath(path, vpath string) (string, string) {
	NoVpath := len(vpath) == 0
	var dir, file string

	// remove basedir prefix from path
	if strings.HasPrefix(path, p.basedir.Directory) {
		path = path[len(p.basedir.Directory):]
	} else {
		panic("path isn't prefixed by the basedir")
	}

	// path is definitely an image file
	if NoVpath {
		dir, file = filepath.Split(path)
		dir = filepath.Clean(dir)
		return dir, file
	}

	// path is an archive
	return filepath.Clean(path), vpath

}

// This is a callback function for archivewalk,
// loads and resizes images, then waits for
func (p *ImageProcessor) Handler(path, vpath string, file io.Reader, d fs.DirEntry, threadID int) error {
	var ext string
	vpath_exists := (len(vpath) > 0)
	if vpath_exists {
		ext = getExt(vpath)
	} else {
		ext = getExt(path)
	}

	db := p.dbConnections.GetResource(threadID)

	parentDir, fileName := p.archiveWalkerPathToDatabasePath(path, vpath)

	// skip if already in DB
	// todo: skip existing as a configuration option rather than presumption
	// todo: debug this
	matchedImage, err := db.MatchImagesByPath(parentDir, fileName, p.basedir.ID, 1, 0)
	if err != nil {
		return err
	}
	if len(matchedImage) > 0 {
		if matchedImage[0].Aesthetic.Valid {
			return nil
		}
	}

	//todo: support all image formats trivially possible.
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
		return fmt.Errorf("error while loading image file: %s:%s: %w", path, vpath, err)
	}

	dbImg := Image{
		Width:     int64(img.Bounds().Dx()),
		Height:    int64(img.Bounds().Dy()),
		BasedirID: int64(p.basedir.ID),
	}
	dbImg.Path = parentDir
	dbImg.SubPath = fileName
	// if it's already in the database then this is an update
	if len(matchedImage) == 1 {
		dbImg.ID = matchedImage[0].ID
	}

	// get embeddings
	if max(dbImg.Width, dbImg.Height) > MAXIMAGESIZE {
		img = imageutil.ScaleImageRGBA(img, MAXIMAGESIZE)
	}
	imgBytes, err := imageutil.ImageToPNG(img)
	if err != nil {
		return fmt.Errorf("error converting image to png: %s:%s: %w", path, vpath, err)
	}

	emb, err := p.apiServer.GetImageEmbedding(imgBytes)
	if err != nil {
		return err
	}
	dbImg.Aesthetic.Float64 = float64(emb.Aesthetic)
	dbImg.Aesthetic.Valid = true
	id, err := db.CreateUpdateImage(&dbImg)
	if err != nil {
		return fmt.Errorf("error adding image to database: %s:%s: %w", path, vpath, err)
	}
	err = db.CreateUpdateEmbedding(id, emb.Embedding)
	if err != nil {
		return fmt.Errorf("error adding image's embedding to database: %s:%s: %w", path, vpath, err)
	}
	return nil
}
