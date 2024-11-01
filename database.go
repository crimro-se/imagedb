package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/jmoiron/sqlx"
)

//go:embed schema.sql
var dbSchema string

type Image struct {
	ID      int64          `db:"rowid"`
	Path    string         `db:"relative_path"`
	SubPath sql.NullString `db:"sub_path"`
	Tags    sql.NullString `db:"tags"`
}

type Database struct {
	con *sqlx.DB
}

// obtain a new sqlx connection.
// may optionally execute the schema
// don't share the same connection to multiple threads
func NewDatabase(file string, execSchema bool) (*Database, error) {
	var myself Database
	var err error
	myself.con, err = sqlx.Connect("sqlite3", file)
	if err != nil {
		return nil, err
	}
	if execSchema {
		_, err = myself.con.Exec(dbSchema)
	}
	return &myself, err
}

// creates or updates embedding for specified Image.
// img ID must be correct.
func (s *Database) CreateUpdateEmbedding(img *Image, emb []float32) error {
	embedding, err := sqlite_vec.SerializeFloat32(emb)
	if err != nil {
		return err
	}
	_, err = s.con.Exec("INSERT OR REPLACE INTO embeddings (rowid, embedding) VALUES (?, ?)", img.ID, embedding)
	return err
}

// useID - will set rowid in query if true, otherwise it's left to fate.
// be careful with this.
func (s *Database) CreateUpdateImage(img *Image) error {
	var err error
	if img.ID > 0 {
		_, err = s.con.NamedExec("INSERT OR REPLACE INTO images (rowid, relative_path, sub_path, tags) VALUES (:rowid, :relative_path, :sub_path, :tags)", img)
	} else {
		_, err = s.con.NamedExec("INSERT OR REPLACE INTO images (relative_path, sub_path, tags) VALUES (:relative_path, :sub_path, :tags)", img)
	}
	return err
}

// returns some images from the db
func (s *Database) ReadImages(limit int) ([]Image, error) {
	imgs := make([]Image, 0)
	err := s.con.Select(&imgs, "SELECT rowid,* FROM images ORDER BY relative_path LIMIT ?", limit)
	return imgs, err
}

// search path
// TODO: fts5
func (s *Database) MatchImages(search string, limit int) ([]Image, error) {
	imgs := make([]Image, 0)
	search = "%" + search + "%"
	err := s.con.Select(&imgs, "SELECT rowid,* FROM images WHERE relative_path LIKE ? OR sub_path LIKE ? LIMIT ?", search, search, limit)
	return imgs, err
}

// embedding search function
// note/todo: currently joining with the vector virtual table doesn't seem to work, so implemented as two queries for now.
func (s *Database) MatchEmbeddings(target []float32, limit int) ([]Image, error) {
	if limit <= 0 {
		return nil, errors.New("limit <= 0, what are you doing?")
	}
	images := make([]Image, 0)
	//rowids := make([]int64, 0)
	targetEmbedding, err := sqlite_vec.SerializeFloat32(target)
	if err != nil {
		return images, err
	}

	//todo perf & accuracy check these.
	//nb: can't join with the virtual table yet.
	err = s.con.Select(&images, "WITH emb AS (SELECT rowid,distance FROM embeddings WHERE embedding match ? order by distance LIMIT ?) SELECT images.rowid,images.* FROM images, emb WHERE images.rowid = emb.rowid ORDER BY emb.distance ASC", targetEmbedding, limit)
	//err = s.con.Select(&images, "select rowid,* FROM images WHERE images.rowid IN (SELECT rowid FROM embeddings WHERE embedding match ? order by distance LIMIT ?)", targetEmbedding, limit)
	return images, err

}

/*
	for row.Next() {
		var i int64
		err = row.Scan(&i)
		if err != nil {
			return images, err
		}
		rowids = append(rowids, i)
	}
	if len(rowids) > 0 {
		// nb: query parameter substitution seems unfamiliar with the array syntax.
		err = s.con.Select(&images, "select rowid,* FROM images where images.rowid IN ("+arrayToString(rowids, ",")+")")
	}
	return images, err
}
*/

func arrayToString(a []int64, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

func (s *Database) Close() {
	s.con.Close()
}
