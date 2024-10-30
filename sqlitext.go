package main

// this loads the sqlite3_vec ext.
// trivial now we have official support.

// i don't know why linking with m isn't enabled by default, so we add that here.

// #cgo LDFLAGS: -lm
// #cgo CFLAGS: -mavx -mavx2 -O3
import "C"

import (
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

// reminder: init functions run automatically
func init() {
	sqlite_vec.Auto()
}

/* This comment is how to load the extension from a .so/.dll
	currently we use the official bindings instead!

// #cgo LDFLAGS: -lm
import "C"

import (
	"database/sql"

	"github.com/mattn/go-sqlite3"
)

// install a db engine /w extensions loaded
func init() {
	sql.Register("sqlite3-vec",
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				if err := conn.LoadExtension("./vec0", "sqlite3_vec_init"); err != nil {
					return err
				}
				return nil
			},
		})
}
*/
