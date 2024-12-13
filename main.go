package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	archivewalk "github.com/crimro-se/imagedb/archivewalk"
)

func main() {
	ctx := context.Background()
	dbfilename := "file.sqlite"
	errCh := make(chan error, 5)
	defer close(errCh)
	go func() {
		for err := range errCh {
			fmt.Println(err)
		}
	}()

	// create db if needed
	db, err := NewDatabase(dbfilename, true)
	if err != nil {
		fmt.Println(err)
		return
	}
	db.Close()
	ip, err := NewImageProcessor(dbfilename, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	ip.RunResultsProcessor(ctx, errCh)

	aw := archivewalk.NewArchiveWalker(10, errCh, true, true, ip.Handler)
	aw.Walk("test_data/valid", ctx)
	time.Sleep(5000 * time.Millisecond)
}

// returns the file extension in lower-case.
// todo: special case for .tar.xz etc maybe.
func getExt(path string) string {
	splitName := strings.Split(path, ".")
	ext := strings.ToLower(splitName[len(splitName)-1])
	return ext
}
