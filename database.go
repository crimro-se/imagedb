package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"reflect"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/jmoiron/sqlx"
)

//go:embed schema.sql
var dbSchema string

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
}

// this is an enum
type SortOrder int

const (
	OrderByPathDesc SortOrder = iota
	OrderByPathAsc
	OrderByAestheticDesc
	OrderByAestheticAsc
)

var (
	sqlImageFields     = mustStructToSQLString(Image{}, []string{})
	sqlImageFieldsNoID = mustStructToSQLString(Image{}, []string{"rowid"})
)

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
// img.ID must be correct.
// todo: vec_quantize_float16 when it works.
func (s *Database) CreateUpdateEmbedding(img *Image, emb []float32) error {
	embedding, err := sqlite_vec.SerializeFloat32(emb)
	if err != nil {
		return err
	}
	_, err = s.con.Exec(`
	INSERT OR REPLACE INTO embeddings 
		   (rowid, embedding) 
	VALUES (?, ?)`, img.ID, embedding)
	return err
}

// Creates or updates an img in the database.
// if image.ID is 0 it'll be automatically set.
// (this is fine as sqlite rowids start at 1)
// TODO: api change, consider returning the ID chosen by the database.
func (s *Database) CreateUpdateImage(img *Image) error {
	var err error
	if img.ID > 0 {
		_, err = s.con.NamedExec(`
		INSERT OR REPLACE INTO images `+sqlImageFields, img)
	} else {
		_, err = s.con.NamedExec(`
		INSERT INTO images `+sqlImageFieldsNoID, img)
	}
	return err
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
func (s *Database) MatchImagesByPath(parent_path, sub_path string, limit, offset int) ([]Image, error) {
	imgs := make([]Image, 0)
	var err error

	if len(sub_path) > 0 {
		err = s.con.Select(&imgs, `
		SELECT rowid,* FROM images
		WHERE parent_path = ? AND
			sub_path = ?
		`+sortOrderToQuery(OrderByPathAsc)+`
		LIMIT ? OFFSET ?`, parent_path, sub_path, limit, offset)
	} else {
		err = s.con.Select(&imgs, `
		SELECT rowid,* FROM images
		WHERE parent_path = ? 
		`+sortOrderToQuery(OrderByPathAsc)+`
		LIMIT ? OFFSET ?`, parent_path, limit, offset)
	}

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

	//nb: can't join with the virtual table.
	err = s.con.Select(&images, `
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
		ORDER BY emb.distance ASC`, targetEmbedding, limit)

	return images, err

}

func (s *Database) Close() {
	s.con.Close()
}

func sortOrderToQuery(so SortOrder) string {
	switch so {
	case OrderByAestheticDesc:
		return "ORDER BY aesthetic DESC"
	case OrderByAestheticAsc:
		return "ORDER BY aesthetic ASC"
	case OrderByPathDesc:
		return "ORDER BY parent_path, sub_path DESC"
	case OrderByPathAsc:
		return "ORDER BY parent_path, sub_path ASC"
	}
	return ""
}

// builds a string of the form "(col1, col2, ...) VALUES (:col1, :col2, ...)"
// based on the tagged 'db' fields in the input struct
// !panics on error
func mustStructToSQLString(input any, ignoreFields []string) string {
	val := reflect.ValueOf(input)
	if val.Kind() != reflect.Struct {
		panic(fmt.Errorf("input must be a struct"))
	}

	columns := make([]string, 0)
	values := make([]string, 0)

	// go doesn't have a Set type, so...
	ignoreSet := make(map[string]struct{})
	for _, field := range ignoreFields {
		ignoreSet[field] = struct{}{}
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		dbTag, ok := field.Tag.Lookup("db")
		if !ok || dbTag == "" {
			continue
		}
		if _, ignored := ignoreSet[dbTag]; ignored {
			continue
		}

		columns = append(columns, dbTag)
		values = append(values, fmt.Sprintf(":%s", dbTag))
	}

	columnString := strings.Join(columns, ", ")
	valueString := strings.Join(values, ", ")

	sqlString := fmt.Sprintf("(%s) VALUES (%s)", columnString, valueString)
	return sqlString
}
