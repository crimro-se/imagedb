package main

import (
	_ "embed"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestSQLX(t *testing.T) {
	//ctx := context.Background()
	db, err := sqlx.Connect("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(dbSchema)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`insert into basedir (directory) values('/');`)
	if err != nil {
		t.Fatal(err)
	}
	im2 := Image{BasedirID: 1, Width: 1, Height: 1, FileSize: 1, Path: "lol/dir", SubPath: "file.png"}
	_, err = db.NamedExec(`insert into images 
			(basedir_id, parent_path, sub_path, width, height, filesize) 
	values(:basedir_id, :parent_path, :sub_path, :width, :height, :filesize)`, im2)
	if err != nil {
		t.Fatal(err)
	}
	var im Image
	err = db.Get(&im, "SELECT * FROM images LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
}

func TestDatabase(t *testing.T) {
	db, err := NewDatabase(":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.con.DB.Exec(`insert into basedir (directory) values('/');`)
	if err != nil {
		t.Fatal(err)
	}
	imgBest := Image{ID: 10, Path: "Best Embedding", BasedirID: 1, Width: 1, Height: 1, FileSize: 1}
	img1 := Image{ID: 1, Path: "Worst Embedding", BasedirID: 1, Width: 1, Height: 1, FileSize: 1}
	err = db.CreateUpdateImage(&img1)
	if err != nil {
		t.Fatal(err)
	}
	tst, err := db.MatchImagesByPath("Worst Embedding", "", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tst) != 1 {
		t.Fail()
	}
	err = db.CreateUpdateImage(&imgBest)
	if err != nil {
		t.Fatal(err)
	}
	imgs, err := db.ReadImages(3, 0, OrderByPathDesc)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Fail()
	}

	emb := make([]float32, 768)
	emb[0] = 1
	emb[1] = 1
	emb[3] = 2
	err = db.CreateUpdateEmbedding(&imgBest, emb)
	if err != nil {
		t.Fatal(err)
	}

	emb[0] = 1
	emb[1] = 1
	emb[3] = 1
	err = db.CreateUpdateEmbedding(&img1, emb)
	if err != nil {
		t.Fatal(err)
	}

	emb[0] = 1
	emb[1] = 1
	emb[3] = 1.9
	imgs2, err := db.MatchEmbeddings(emb, 2)
	if err != nil {
		t.Fatal(err)
	}
	if imgs2[0].ID != 10 {
		t.Fail()
	}

	db.Close()
}

func TestSQLStrings(t *testing.T) {
	im := Image{BasedirID: 1, Width: 1, Height: 1, FileSize: 1, Path: "lol/dir", SubPath: "file.png"}
	sqlstr := mustStructToSQLString(im, []string{"rowid"})
	if len(sqlstr) < 1 {
		t.Fail()
	}
}
