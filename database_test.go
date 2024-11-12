package main

import (
	_ "embed"
	"fmt"
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
	var im2 Image
	im2.Path = "/lol"
	_, err = db.NamedExec("insert into images (relative_path) values(:relative_path)", im2)
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
	imgBest := Image{ID: 10, Path: "Best Embedding"}
	img1 := Image{ID: 1, Path: "Worst Embedding"}
	err = db.CreateUpdateImage(&img1)
	if err != nil {
		t.Fatal(err)
	}
	err = db.CreateUpdateImage(&imgBest)
	if err != nil {
		t.Fatal(err)
	}
	imgs, err := db.MatchImages("Embedding", 3)
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
	fmt.Println((imgs2))

	db.Close()
}
