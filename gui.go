package main

/*

var (
	ctx        context.Context
	cancelFunc context.CancelFunc
	db         *Database
	ip         *ImageProcessor
	errCh      chan error
	aw         *archivewalk.ArchiveWalk
)

func main() {
	a := app.New()
	w := a.NewWindow("Image Database Application")

	dbfilename := "file.sqlite"
	var err error

	ctx, cancelFunc = context.WithCancel(context.Background())
	errCh = make(chan error, 5)
	go func() {
		for err := range errCh {
			dialog.ShowError(err, w)
		}
	}()

	db, err = NewDatabase(dbfilename, true)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	defer db.Close()

	ip, err = NewImageProcessor(dbfilename, 0)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	ip.RunResultsProcessor(ctx, errCh)

	startButton := widget.NewButton("Start Processing", func() {
		if aw == nil {
			aw = archivewalk.NewArchiveWalker(10, errCh, true, true, ip.Handler)
		}
		go aw.Walk("test_data/valid", ctx)
	})

	stopButton := widget.NewButton("Stop Processing", func() {
		cancelFunc()
		ctx, cancelFunc = context.WithCancel(context.Background())
		ip.RunResultsProcessor(ctx, errCh)
	})

	imageList := widget.NewMultiLineEntry()

	updateImageList := func() {
		imgs, err := db.ReadImages(10, 0, OrderByPathAsc)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		var sb strings.Builder
		for _, img := range imgs {
			sb.WriteString(fmt.Sprintf("ID: %d, Path: %s/%s\n", img.ID, img.Path, img.SubPath))
		}
		imageList.SetText(sb.String())
	}

	updateButton := widget.NewButton("Update Image List", updateImageList)

	content := container.NewVBox(
		widget.NewLabel("Image Database Application"),
		startButton,
		stopButton,
		updateButton,
		widget.NewSeparator(),
		widget.NewLabel("Recent Images:"),
		imageList,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(600, 400))
	w.ShowAndRun()
}



type ImageData struct {
	dbData *Image
	Image  *canvas.Image
}

type ImageViewerCallback func(dbData Image)

// can display
type ImageViewerManager struct {
	mu           sync.Mutex
	images       []ImageData
	onClickFunc  ImageViewerCallback
	layout       *fyne.Container
	imageButtons fyne.Widget
}

func NewImageViewerManager() {
	var ivm ImageViewerManager
	ivm.layout = container.NewGridWrap(fyne.NewSize(128, 128))
}

func (ivm *ImageViewerManager) Add(img image.Image, dbdata Image) {
	image := canvas.NewImageFromImage(img)
	id := ImageData{
		dbData: &dbdata,
		Image:  image,
	}
	ivm.images = append(ivm.images, id)
	fyne.NewStaticResource()
	ivm.layout.Add(image)
}

*/

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/crimro-se/imagedb/archivewalk"
)

type ImageList struct {
	*fyne.Container
	callback func(Image)
}

// The GUI element we use to display many images, typically query results.
func NewImageList(clickCallback func(Image)) *ImageList {
	il := ImageList{
		callback:  clickCallback,
		Container: container.NewGridWrap(fyne.NewSize(128, 128)), // TODO: de-hardcode this
	}
	return &il
}

func (il *ImageList) AddImage(img image.Image, dbdata Image) {
	imgBtn := NewImageButtonFromImage(img, dbdata, il.callback)
	il.Add(imgBtn)
}

func (il *ImageList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(il.Container)
}

// an image button+data package. The data set is also available on click
type ImageButtonWithData[T any] struct {
	*canvas.Image
	onClick func(T)
	data    T
}

func NewImageButtonFromImage[T any](img image.Image, data T, onClick func(T)) *ImageButtonWithData[T] {
	ib := ImageButtonWithData[T]{
		onClick: onClick,
		Image:   canvas.NewImageFromImage(img),
		data:    data,
	}
	return &ib
}

func (ib *ImageButtonWithData[T]) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ib.Image)
}

func (ib *ImageButtonWithData[T]) Tapped(ev *fyne.PointEvent) {
	ib.onClick(ib.data)
}

type GUI struct {
	window   fyne.Window
	db       *Database
	actables []*widget.DisableableWidget

	guiBasedirs   *fyne.Container //vbox container
	basedirsState map[int64]binding.Bool

	imageList *ImageList

	indexingDialogue *ImageProcessDialogue
	active           bool
}

func NewGUI(window fyne.Window, db *Database) *GUI {
	gui := GUI{
		window:   window,
		db:       db,
		actables: make([]*widget.DisableableWidget, 0),

		basedirsState: make(map[int64]binding.Bool),
		guiBasedirs:   container.NewVBox(),
	}
	return &gui
}

// deactivates all UI elements
func (gui *GUI) deactivateAll() {
	gui.active = false
	for _, w := range gui.actables {
		w.Disable()
	}
}

// activates all UI elements
func (gui *GUI) activateAll() {
	gui.active = true
	for _, w := range gui.actables {
		w.Enable()
	}
}

// resets the list of indexes in the GUI w.r.t the Database.
// existing toggle states are preserved
// TODO: states should be saved
func (gui *GUI) rebuildBasedirs() {
	basedirs, err := gui.db.GetAllBasedir()
	if err != nil {
		return // TODO: Log
	}
	gui.guiBasedirs.RemoveAll()
	oldstates := gui.basedirsState
	gui.basedirsState = make(map[int64]binding.Bool)
	for _, basedir := range basedirs {
		oldstate, ok := oldstates[basedir.ID]
		if !ok {
			oldstate = binding.NewBool()
		}
		check := widget.NewCheckWithData(midTruncateString(basedir.Directory, 25), oldstate)
		gui.basedirsState[basedir.ID] = oldstate
		gui.guiBasedirs.Add(check)
	}
}

func midTruncateString(str string, maxlen int) string {
	const trimPosition = 10 // X characters before the end of string
	const trimStr = "[…]"
	strlen := len(str)

	if len(str) <= maxlen {
		return str
	}
	if maxlen < trimPosition {
		return str[:maxlen-len(trimStr)] + trimStr
	}
	prefix := str[:(maxlen - (trimPosition + len(trimStr)))]
	return prefix + trimStr + str[strlen-trimPosition:]
}

// list of basedir IDs that the user has enabled in the UI
func (gui *GUI) getActiveBasedirsID() []int64 {
	ints := make([]int64, 0)
	for id, val := range gui.basedirsState {
		b, err := val.Get()
		if err == nil {

			if b {
				ints = append(ints, id)
			}
		} else {
			// TODO: Log err
		}

	}
	return ints
}

// list of Basedir that the user has enabled in the UI
func (gui *GUI) getActiveBasedirs() []Basedir {
	filteredBasedirs := make([]Basedir, 0)
	activeIDs := gui.getActiveBasedirsID()
	allBasedirs, err := gui.db.GetAllBasedir()
	//TODO: log err
	if err != nil {
	}
	for _, bd := range allBasedirs {
		if slices.Contains(activeIDs, bd.ID) {
			filteredBasedirs = append(filteredBasedirs, bd)
		}
	}
	return filteredBasedirs
}

func (gui *GUI) buildIndexButtons() *fyne.Container {
	addIndexBtn := widget.NewButton("New", func() {
		basedirSelector := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil {
				dialog.NewError(err, gui.window).Show()
				return
			}
			if lu == nil {
				return
			}
			path := lu.Path()
			if len(path) <= 0 {
				dialog.NewError(fmt.Errorf("selected directory has no path"), gui.window).Show()
				return
			}

			err = gui.db.CreateBasedir(path)
			if err != nil {
				dialog.NewError(err, gui.window).Show()
				return
			}
			gui.rebuildBasedirs()
			// Todo: start indexing
		}, gui.window)
		basedirSelector.Resize(gui.window.Canvas().Size())
		basedirSelector.Show()
	})

	updateIndexBtn := widget.NewButton("Update", func() {
		activeBasedirs := gui.getActiveBasedirs()
		if len(activeBasedirs) != 1 {
			dialog.NewInformation("", "Select only exactly one index first", gui.window).Show()
			return
		}
		gui.indexingDialogue.Show("db.sqlite", activeBasedirs[0])
	})

	deleteIndexBtn := widget.NewButton("Delete", func() {
		activeBasedirs := gui.getActiveBasedirs()
		if len(activeBasedirs) != 1 {
			dialog.NewInformation("", "Select only exactly one index first", gui.window).Show()
			return
		}

		gui.deactivateAll()
		err := gui.db.DeleteBasedir(activeBasedirs[0].ID)
		if err != nil {
			dialog.NewError(err, gui.window).Show()
		}
		gui.rebuildBasedirs()
		gui.activateAll()
	})

	gui.actables = append(gui.actables, &addIndexBtn.DisableableWidget)
	gui.actables = append(gui.actables, &updateIndexBtn.DisableableWidget)
	gui.actables = append(gui.actables, &deleteIndexBtn.DisableableWidget)
	indexesButtons := container.NewHBox(addIndexBtn, updateIndexBtn, deleteIndexBtn)
	return indexesButtons
}

func loadPNGFromFile(filePath string) (image.Image, error) {
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Decode the image
	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// assembles the main gui window and wires all the components on it
func (gui *GUI) Build() {
	// LEFT ----------------------------------------------------
	indexesLabel := widget.NewLabel("Indexes")
	gui.rebuildBasedirs()
	// new basedir button & implementation

	indexesButtons := gui.buildIndexButtons()

	imgInfoLabel := widget.NewLabel("Image Info")
	appLogLabel := widget.NewLabel("Log")

	imgInfo := widget.NewRichText()
	appLog := widget.NewRichText()
	appLog.AppendMarkdown("**Started**")
	leftContainer := container.NewVBox(
		indexesLabel, gui.guiBasedirs, indexesButtons,
		imgInfoLabel, imgInfo,
		appLogLabel, appLog,
	)
	// RIGHT ----------------------------------------------------
	searchbox := widget.NewEntry()
	gui.imageList = NewImageList(func(im Image) {
		// TODO: real handler
		fmt.Println("Test")
	})
	rightContainer := container.NewVBox(searchbox, gui.imageList)

	// DIALOGUES ---------------------------------------------------
	gui.indexingDialogue = NewImageProcessDialogue(gui.window)

	total := container.NewHSplit(leftContainer, rightContainer)
	gui.window.SetContent(total)
	gui.window.Resize(fyne.NewSquareSize(600))
}

type ImageProcessDialogue struct {
	*dialog.CustomDialog
	processor     *ImageProcessor
	displayedPath binding.String
	basedir       Basedir
	ctx           context.Context
	ctxCancel     context.CancelFunc
}

// prepares the dialogue for use then shows it
func (ipd *ImageProcessDialogue) Show(dbfile string, basedir Basedir) error {
	var err error
	ipd.basedir = basedir
	ipd.displayedPath.Set(basedir.Directory)
	ipd.processor, err = NewImageProcessor(dbfile, basedir)
	if err != nil {
		return err
	}
	ipd.ctx, ipd.ctxCancel = context.WithCancel(context.Background())
	ipd.CustomDialog.Show()
	return nil
}

// a dialogue that handles directory walking and indexing
func NewImageProcessDialogue(w fyne.Window) *ImageProcessDialogue {
	content := container.NewVBox()
	ipd := &ImageProcessDialogue{
		CustomDialog:  dialog.NewCustomWithoutButtons("Indexing", content, w),
		displayedPath: binding.NewString(),
	}
	pathLabel := container.NewHBox()
	pathLabel.Add(widget.NewLabel("Path: "))
	pathLabel.Add(widget.NewLabelWithData(ipd.displayedPath))
	content.Add(pathLabel)

	// api server queue
	queueStatus := binding.NewString()
	queueBox := widget.NewLabelWithData(queueStatus)
	queueLabel := widget.NewLabel("Queue Status: ")
	content.Add(container.NewHBox(queueLabel, queueBox))

	//log
	logBox := widget.NewMultiLineEntry()
	logBox.Append("Log:\n")
	content.Add(logBox)
	errCh := make(chan error, 5)
	go func() {
		for err := range errCh {
			logBox.Append(err.Error() + "\n")
			logBox.CursorRow = 0xFFFFFFFFFFFFFF
		}
	}()

	// nb: various variables referenced here are set by Show()
	var startBtn *widget.Button
	startBtn = widget.NewButton("Start", func() {
		startBtn.Disable()
		go func() {
			aw := archivewalk.NewArchiveWalker(6, errCh, true, true, ipd.processor.Handler)
			aw.Walk(ipd.basedir.Directory, ipd.ctx)
			logBox.Append("Done\n")
		}()
	})
	content.Add(startBtn)
	content.Add(widget.NewButton("Cancel", func() {
		startBtn.Enable()
		ipd.ctxCancel()
		ipd.processor = nil
		ipd.CustomDialog.Hide()
	}))
	return ipd
}

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
	w.ShowAndRun()

}
