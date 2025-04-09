package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"os"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/crimro-se/imagedb/internal/imagedbutil"
	"github.com/crimro-se/imagedb/querystructs"
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
	ID          int64           `db:"rowid"`
	BasedirID   int64           `db:"basedir_id"`
	BasedirPath string          // obtain via foreign key if needed.
	Path        string          `db:"parent_path"` // path of parent directory or zip file
	SubPath     string          `db:"sub_path"`    // filename or path within zip
	Tags        sql.NullString  `db:"tags"`
	Aesthetic   sql.NullFloat64 `db:"aesthetic"`
	Width       int64           `db:"width"`
	Height      int64           `db:"height"`
	FileSize    int64           `db:"filesize"`
}

// todo: handle archives
// the image's BasedirPath needs to be set first
func (dbImg *Image) GetRealPath() string {
	return imagedbutil.AddTrailingSlash(dbImg.BasedirPath) + imagedbutil.AddTrailingSlash(dbImg.Path) + dbImg.SubPath
}

// todo: handle archives
// the image's BasedirPath needs to be set first
func (dbImg *Image) Load() (image.Image, error) {
	// Open the file for reading
	filePath := dbImg.GetRealPath()
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Decode the image
	imgImg, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return imgImg, nil

}

type Database struct {
	con                  *sqlx.DB
	whereClauseGenerator func(QueryFilter) (string, error)
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
	if err != nil {
		return &myself, err
	}
	myself.whereClauseGenerator, err = querystructs.BuildWhereClauseGenerator(QueryFilter{})
	return &myself, err
}

func (s *Database) AugmentImages(images []Image) ([]Image, error) {
	basedirs, err := s.GetAllBasedirAsMap()
	if err != nil {
		return nil, err
	}
	for i := range images {
		images[i].BasedirPath = basedirs[images[i].BasedirID]
	}
	return images, nil
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

func (s *Database) GetAllBasedirAsMap() (map[int64]string, error) {
	basedMap := make(map[int64]string, 0)
	bds, err := s.GetAllBasedir()
	if err != nil {
		return nil, err
	}
	for _, db := range bds {
		basedMap[db.ID] = db.Directory
	}
	return basedMap, nil
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

// returns some images from the db, filtered via Query Filter
// nb: empty query filter won't work, at least set the Limit
func (s *Database) ReadImages(qf QueryFilter, so SortOrder) ([]Image, error) {
	if len(qf.BaseDirs) == 0 {
		return nil, fmt.Errorf("no basedirs specified in query")
	}
	where, err := s.whereClauseGenerator(qf)
	if err != nil {
		return nil, err
	}
	if len(where) > 0 {
		where = "WHERE " + where + " "
	}
	queryString := `
	SELECT rowid,* FROM images ` + where + sortOrderToQuery(so) + `
	LIMIT :limit OFFSET :offset`

	query, args, err := sqlx.Named(queryString, qf)
	if err != nil {
		return nil, err
	}
	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, err
	}
	query = s.con.Rebind(query)

	rows, err := s.con.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	imgs, err := s.scanAllRows(rows)
	return imgs, err
}

func (s *Database) scanAllRows(rows *sqlx.Rows) ([]Image, error) {
	imgs := make([]Image, 0)
	var img Image
	for rows.Next() {
		err := rows.StructScan(&img)
		if err != nil {
			return imgs, err
		}
		imgs = append(imgs, img)
	}
	return imgs, nil
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

func (s *Database) ReadEmbedding(imageRowID int64) ([]byte, error) {
	emb := make([]byte, 0)
	queryString := "SELECT embedding FROM embeddings WHERE rowid = ?"
	rows, err := s.con.Queryx(queryString, imageRowID)
	for rows.Next() {
		err = rows.Scan(&emb)
	}
	return emb, err
}

// nb: target can be produced from sqlite_vec.SerializeFloat32
func (s *Database) MatchEmbeddingsWithFilter(target []byte, qf QueryFilter) ([]Image, error) {
	if qf.Limit <= 0 {
		return nil, errors.New("limit must be greater than zero")
	}
	if len(qf.BaseDirs) == 0 {
		return nil, fmt.Errorf("no basedirs specified in query")
	}

	// Generate WHERE clause for query filter
	where, err := s.whereClauseGenerator(qf)
	if err != nil {
		return nil, err
	}

	// Build the query
	// CTEs because joins on vector table aren't possible.
	queryString := `
		WITH filtered_images AS (
			SELECT rowid
			FROM images
			WHERE ` + where + `
		),
		filtered AS (
			SELECT rowid, distance 
			FROM embeddings
			WHERE embedding MATCH ? AND k = ? AND rowid IN (select rowid FROM filtered_images)
		)
		SELECT images.rowid, images.*
		FROM images, filtered
		WHERE images.rowid = filtered.rowid
		ORDER BY distance ASC`

	// Prepare named query
	namedQuery, args, err := sqlx.Named(queryString, qf)
	if err != nil {
		return nil, err
	}

	// Insert the embedding and limit parameters at the beginning
	//args = append([]interface{}{target, qf.Limit}, args...)
	args = append(args, target, qf.Limit)

	// Handle IN clauses if needed
	namedQuery, args, err = sqlx.In(namedQuery, args...)
	if err != nil {
		return nil, err
	}

	// Rebind for the specific database dialect
	namedQuery = s.con.Rebind(namedQuery)

	// Execute query
	images := make([]Image, 0)
	err = s.con.Select(&images, namedQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to match embeddings with filter: %w", err)
	}

	return images, nil
}

func (s *Database) Close() error {
	return s.con.Close()
}
