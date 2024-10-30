// dirwalk extended to also walk through zip and rar arcvhies.
package archivewalk

import (
	"archive/zip"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nwaples/rardecode/v2"
)

/*
path - path to the real open file
vpath - virtual path to file within an archive. empty string if we're not in an archive
file - a file reader. will be closed after your handler function, so finish reading it before returning.
return an error/nil.

if you wish to early abort, monitor the error channel and close the context yourself.

Be advised that handlers are invoked concurrently from other threads
*/
type FileHandler func(path, vpath string, file io.Reader, d fs.DirEntry) error

type Task struct {
	path     string
	dirEntry fs.DirEntry
}

type ArchiveWalk struct {
	workers          int
	errorCh          chan error
	openZip, openRar bool
	handler          FileHandler
	wg               *sync.WaitGroup
}

// creates a new archive walker with certain settings.
func NewArchiveWalker(workers int, errorCh chan error, openZip bool, openRar bool, handler FileHandler) ArchiveWalk {
	var aw ArchiveWalk
	aw.handler = handler
	aw.workers = workers
	aw.errorCh = errorCh
	aw.openZip = openZip
	aw.openRar = openRar
	aw.wg = &sync.WaitGroup{}
	return aw
}

// walks all files from specified root, including entering supported archives.
// ctx - can halt the dirwalk
// the errorCh channel can optionally be set to recieve errors as they happen.
// Important note: doesn't follow symbolic directory links (to prevent looping)
func (aw *ArchiveWalk) Walk(rootPath string, ctx context.Context) {
	// workers
	tasks := make(chan Task, aw.workers+2)
	aw.createWorkers(tasks, ctx)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		// sanity checks
		if err != nil {
			err = aw.softenError(err)
			return err
		}
		select {
		case <-ctx.Done():
			return filepath.SkipAll
		default:
		}
		if d.IsDir() {
			return nil
		}

		// add to queue
		var task Task
		task.dirEntry = d
		task.path = path
		tasks <- task

		return nil

	})
	aw.softenError(err)
	close(tasks) // signifies no more values to send.
	aw.wg.Wait()
}

// creates ArchiveWalk.workers number of worker threads, listening on taskQueue and added to ArchiveWalk.wg WaitGroup.
func (aw *ArchiveWalk) createWorkers(taskQueue chan Task, ctx context.Context) {
	for i := 0; i < aw.workers; i++ {
		aw.wg.Add(1)
		go func() {
			defer aw.wg.Done()
			//walkerThread(taskQueue, ctx, handler, aw.errorCh, skipArchives)
			aw.work(taskQueue, ctx, aw.handler)
		}()
	}
}

// The actual work of processing archives & files is done here.
// tasks are recieved from taskCh and then handled appropriately by provided
// FileHandler callback.
// can be terminated via ctx or closing the taskCh.
// errors are sent to errorCh if it was set in constructor
// zip & rar handling can optionally be skipped.
func (aw *ArchiveWalk) work(taskCh <-chan Task, ctx context.Context, fn FileHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-taskCh:
			if !ok {
				return
			}

			// walk archives
			ext := getExt(task.dirEntry.Name())
			if aw.openRar && ext == "rar" {
				err := rarWalk(task, fn, ctx)
				notifyIfError(aw.errorCh, err)
				break
			}
			if aw.openZip && ext == "zip" {
				err := zipWalk(task, fn, ctx)
				notifyIfError(aw.errorCh, err)
				break
			}

			// not an archive, so handle file directly.
			f, err := os.Open(task.path)
			if err != nil {
				notifyIfError(aw.errorCh, err)
			} else {
				defer f.Close()
				fn(task.path, "", f, task.dirEntry)
			}
		}
	}
}

// walks .zip archives
// todo: file crc check?
func zipWalk(task Task, fh FileHandler, ctx context.Context) error {
	r, err := zip.OpenReader(task.path)
	if err != nil {
		return err
	}
	defer r.Close()

	//iterate through the archive
	for _, f := range r.File {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		fileHandle, err := f.Open()
		if err == nil {
			defer fileHandle.Close()
			fh(task.path, f.Name, fileHandle, task.dirEntry)
		} else {
			return err
		}
	}
	return nil
}

// walks .rar archives
func rarWalk(task Task, fh FileHandler, ctx context.Context) error {
	r, err := rardecode.OpenReader(task.path)
	if err != nil {
		return err
	}
	defer r.Close()

	//iterate through the archive
	err = nil
	var header *rardecode.FileHeader
	for err == nil {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		header, err = r.Next()
		if err == nil && header != nil {
			if header.IsDir {
				continue
			}
			fh(task.path, header.Name, r, task.dirEntry)
		}
	}

	//wipe EOF error since we shouldn't care about it.
	if err.Error() == "EOF" {
		err = nil
	}
	return err
}

// returns the file extension in lower-case.
// todo: special case for .tar.XX
func getExt(path string) string {
	splitName := strings.Split(path, ".")
	ext := strings.ToLower(splitName[len(splitName)-1])
	return ext
}

// sends the err if it is an error to channel if it is a channel
// (checks neither are nil)
func notifyIfError(errorCh chan error, err error) {
	if err != nil && errorCh != nil {
		errorCh <- err
	}
}

// errors such as filepath.SkipAll or filepath.SkipDir are returned,
// any other error is sent to errorCh, and nil is returned.
func (aw *ArchiveWalk) softenError(err error) error {
	if err == filepath.SkipAll || err == filepath.SkipDir {
		return err
	}
	notifyIfError(aw.errorCh, err)
	return nil
}

/*
 Future plans:
 - pause/resume support maybe.
 - multi-part archive testing
 - does archive decoding actually check file hashes?
*/
