package main

import (
	"fmt"

	"fyne.io/fyne/v2/app"
)

func main() {
	a := app.NewWithID("crimro-se/imagedb")
	w := a.NewWindow("imagedb")
	db, err := NewDatabase("db.sqlite", true)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	conf, err := LoadConfig("config.ini")
	if err != nil {
		fmt.Println(err)
		// nb: should be safe to continue regardless
	}

	gui := NewGUI(w, db, conf)
	_ = gui
	w.ShowAndRun()

}
