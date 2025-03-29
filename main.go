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
	gui := NewGUI(w, db)
	gui.Build()
	gui.showSomething()
	w.ShowAndRun()

}
