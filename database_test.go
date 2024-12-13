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
	// nb: id 0 should be changed by the insert function.
	img1 := Image{ID: 0, Path: "Worst Embedding", BasedirID: 1, Width: 1, Height: 1, FileSize: 1}

	// Capture the ID returned by CreateUpdateImage
	id1, err := db.CreateUpdateImage(&img1)
	if err != nil {
		t.Fatal(err)
	}
	img1.ID = id1 // Update img1 with the returned ID

	tst, err := db.MatchImagesByPath("Worst Embedding", "", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tst) != 1 {
		t.Fail()
	}

	idBest, err := db.CreateUpdateImage(&imgBest)
	if err != nil {
		t.Fatal(err)
	}
	imgBest.ID = idBest // Update imgBest with the returned ID

	imgs, err := db.ReadImages(3, 0, OrderByPathDesc)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Fail()
	}

	// Additional checks to ensure the IDs are correct
	if img1.ID != 1 {
		t.Errorf("Expected img1 ID to be 1, got %d", img1.ID)
	}
	if imgBest.ID != 10 {
		t.Errorf("Expected imgBest ID to be 10, got %d", imgBest.ID)
	}

	emb := make([]float32, 768)
	emb[0] = 1
	emb[1] = 1
	emb[3] = 2
	err = db.CreateUpdateEmbedding(imgBest.ID, emb)
	if err != nil {
		t.Fatal(err)
	}

	emb[0] = 1
	emb[1] = 1
	emb[3] = 1
	err = db.CreateUpdateEmbedding(img1.ID, emb)
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
	sqlstr, err := structToSQLString(im, []string{"rowid"})
	if err != nil {
		t.Error(err)
	}
	if len(sqlstr) < 1 {
		t.Fail()
	}
}
