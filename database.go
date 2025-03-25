package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var dbSchema string

type Basedir struct {
	ID        int64  `db:"rowid"`
	Directory string `db:"directory"`
}

type Image struct {
	ID        int64           `db:"rowid"`
	BasedirID int64           `db:"basedir_id"`
	Path      string          `db:"parent_path"`
	SubPath   string          `db:"sub_path"`
	Tags      sql.NullString  `db:"tags"`
	Aesthetic sql.NullFloat64 `db:"aesthetic"`
	Width     int64           `db:"width"`
	Height    int64           `db:"height"`
	FileSize  int64           `db:"filesize"`
}

type Database struct {
	con *sqlx.DB
	// pre-calculated strings for use in queries
	insertIntoImageTableSQL     string
	insertIntoImageTableSQLNoID string
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
	if err != nil {
		return &myself, err
	}
	myself.insertIntoImageTableSQL, err = structToSQLString(Image{}, []string{})
	if err != nil {
		return &myself, err
	}
	myself.insertIntoImageTableSQLNoID, err = structToSQLString(Image{}, []string{"rowid"})
	return &myself, err
}

func (s *Database) CreateBasedir(directory string) error {
	_, err := s.con.Exec(`
	INSERT INTO basedir 
		   (directory) 
	VALUES (?)`, directory)
	return err
}

func (s *Database) GetAllBasedir() ([]Basedir, error) {
	based := make([]Basedir, 0)
	err := s.con.Select(&based, `SELECT rowid,* FROM basedir`)
	return based, err
}

func (s *Database) DeleteBasedir(id int64) error {
	_, err1 := s.con.Exec(`DELETE FROM basedir WHERE rowid = ?`, id)
	err2 := s.DeleteImagesByBasedirID(id)
	return errors.Join(err1, err2)
}

// removes images and associated embeddings by basedir_id
func (s *Database) DeleteImagesByBasedirID(id int64) error {
	_, err1 := s.con.Exec(`
	DELETE FROM embeddings 
	WHERE rowid IN 
		(SELECT rowid FROM images WHERE basedir_id = ?)`, id)

	_, err2 := s.con.Exec(`
	DELETE FROM images 
		WHERE images.basedir_id = ?`, id)
	return errors.Join(err1, err2)
}

// creates or updates embedding for specified Image.
// img.ID must be correct.
// todo: vec_quantize_float16 when it works.
func (s *Database) CreateUpdateEmbedding(imgID int64, emb []float32) error {
	embedding, err := sqlite_vec.SerializeFloat32(emb)
	if err != nil {
		return err
	}
	_, err = s.con.Exec(`
	INSERT OR REPLACE INTO embeddings 
		   (rowid, embedding) 
	VALUES (?, ?)`, imgID, embedding)
	return err
}

func (s *Database) UpdateAesthetic(imgID int64, aesthetic float32) error {
	_, err := s.con.Exec(`
	UPDATE images SET aesthetic = ?
	WHERE rowid = ?`, aesthetic, imgID)
	return err
}

// Creates or updates an img in the database.
// If image.ID is 0, it'll be automatically set.
// (this is fine as SQLite rowids start at 1)
// Returns the ID of the image chosen by the database.
func (s *Database) CreateUpdateImage(img *Image) (int64, error) {
	var err error
	if img.ID > 0 {
		_, err = s.con.NamedExec(`
			INSERT OR REPLACE INTO images `+s.insertIntoImageTableSQL, img)
		return img.ID, err
	} else {
		result, err := s.con.NamedExec(`
			INSERT INTO images `+s.insertIntoImageTableSQLNoID, img)
		if err != nil {
			return 0, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		img.ID = int64(id)
		return id, nil
	}
}

// returns some images from the db, sorted by path
func (s *Database) ReadImages(limit, offset int, so SortOrder) ([]Image, error) {
	imgs := make([]Image, 0)
	err := s.con.Select(&imgs, `
	SELECT rowid,* 
	FROM images 
    `+sortOrderToQuery(so)+`
	LIMIT ? OFFSET ?`, limit, offset)
	return imgs, err
}

// Finds the image entry in the database with the given path. (exact match)
// May return multiple results for archives if subSearch isn't specified
// TODO: fts5 version
func (s *Database) MatchImagesByPath(parent_path, sub_path string, basedirID int64, limit, offset int) ([]Image, error) {
	imgs := make([]Image, 0)
	var err error

	if len(sub_path) > 0 {
		err = s.con.Select(&imgs, `
		SELECT rowid,* FROM images
		WHERE parent_path = ? AND
			sub_path = ? AND
			basedir_id = ?
		`+sortOrderToQuery(OrderByPathAsc)+`
		LIMIT ? OFFSET ?`, parent_path, sub_path, basedirID, limit, offset)
	} else {
		err = s.con.Select(&imgs, `
		SELECT rowid,* FROM images
		WHERE parent_path = ? AND
			basedir_id = ?
		`+sortOrderToQuery(OrderByPathAsc)+`
		LIMIT ? OFFSET ?`, parent_path, basedirID, limit, offset)
	}

	return imgs, err
}

// embedding search function
func (s *Database) MatchEmbeddings(target []float32, limit int) ([]Image, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be greater than zero")
	}

	targetEmbedding, err := sqlite_vec.SerializeFloat32(target)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize target embedding: %w", err)
	}

	const query = `
		WITH emb AS (
			SELECT rowid,distance 
			FROM embeddings 
			WHERE embedding match ? 
			ORDER BY distance 
			LIMIT ?
		) 
		SELECT images.rowid,images.* 
		FROM images, emb 
		WHERE images.rowid = emb.rowid 
		ORDER BY emb.distance ASC`
	images := make([]Image, 0)
	err = s.con.Select(&images, query, targetEmbedding, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to match embeddings: %w", err)
	}

	return images, err
}

func (s *Database) Close() error {
	return s.con.Close()
}
