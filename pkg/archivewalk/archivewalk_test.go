package archivewalk

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"testing"
	"time"
)

var dummyHandler = func(path, vpath string, file io.Reader, d fs.DirEntry, threadID int) error { return nil }

/*
- Ensures the basic operation of archive walking seems to work
*/
func Test_archivewalk(t *testing.T) {
	ctx := context.Background()
	errCh := make(chan error)
	defer close(errCh)
	errors := 0
	files := 0

	go func() {
		for range errCh {
			errors++
		}
	}()

	// checks for no errors and correct number of images read
	aw := NewArchiveWalker(10, errCh, true, true, func(path, vpath string, file io.Reader, d fs.DirEntry, threadID int) error {
		files++
		return nil
	})
	aw.Walk("../test_data/valid", ctx)
	time.Sleep(10 * time.Millisecond)

	if files != 17 {
		fmt.Println("Incorrect number of files handled")
		t.Fail()
	}

	if errors != 0 {
		fmt.Println("Error has occured when unexpected")
		t.Fail()
	}

	// confirms invalid path results in an error
	aw2 := NewArchiveWalker(10, errCh, false, false, dummyHandler)
	aw2.Walk("../test_data/bad_path", ctx)
	time.Sleep(10 * time.Millisecond)

	if errors < 1 {
		fmt.Println("Bad path didn't raise an error")
		t.Fail()
	}
}

/*
todo: test what happens when handler returns errors
*/
