package main

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/crimro-se/imagedb/archivewalk"
	"github.com/crimro-se/imagedb/imageutil"
	"github.com/skratchdot/open-golang/open"
)

type ImageList struct {
	*fyne.Container
	callback func(*fyne.PointEvent, Image)
}

var THUMBNAIL_SIZE = 256

// The GUI element we use to display many images, typically query results.
func NewImageList(clickCallback func(*fyne.PointEvent, Image)) *ImageList {
	il := ImageList{
		callback:  clickCallback,
		Container: container.NewGridWrap(fyne.NewSquareSize(float32(THUMBNAIL_SIZE))), // TODO: de-hardcode this
	}
	return &il
}

func (il *ImageList) AddImage(img image.Image, dbdata Image) {
	imgBtn := NewImageButtonFromImage(img, dbdata, il.callback)
	//imgBtn.SetMinSize(fyne.NewSquareSize(64))
	imgBtn.Image.FillMode = canvas.ImageFillContain
	//imgBtn.Resize(fyne.NewSquareSize(64))
	il.Add(imgBtn)
	//il.Add(widget.NewButton("test", nil))
}

func (il *ImageList) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(il.Container)
}

// An image button+data package. The data set is also available on click
type ImageButtonWithData[T any] struct {
	widget.BaseWidget // Embed BaseWidget to get proper widget behavior
	Image             *canvas.Image
	onClick           func(*fyne.PointEvent, T)
	data              T
}

func NewImageButtonFromImage[T any](img image.Image, data T, onClick func(*fyne.PointEvent, T)) *ImageButtonWithData[T] {
	ib := &ImageButtonWithData[T]{
		Image:   canvas.NewImageFromImage(img),
		onClick: onClick,
		data:    data,
	}
	ib.ExtendBaseWidget(ib) // Initialize BaseWidget
	ib.Image.FillMode = canvas.ImageFillContain
	return ib
}

// CreateRenderer implements fyne.Widget
func (ib *ImageButtonWithData[T]) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ib.Image)
}

// Tapped implements fyne.Tappable
func (ib *ImageButtonWithData[T]) Tapped(pe *fyne.PointEvent) {
	ib.onClick(pe, ib.data)
}

// MinSize implements fyne.Widget
func (ib *ImageButtonWithData[T]) MinSize() fyne.Size {
	return fyne.NewSize(64, 64) // Set your minimum size here
}

/*
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

*/

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
		check := widget.NewCheckWithData(midTruncateString(basedir.Directory, 36), oldstate)
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

// for convenient use with QueryFilter
func (gui *GUI) getActiveBasedirsString() string {
	basedirs := gui.getActiveBasedirsID()
	var sb strings.Builder
	for i, basedir := range basedirs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.FormatInt(basedir, 10))
	}
	return sb.String()
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

/*
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
*/

/* Display a pop-up menu when an image is clicked (for now, I don't care which mouse button clicked.)
 */
func (gui *GUI) ShowThumbnailMenu(pe *fyne.PointEvent, im Image) {

	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Open Image", func() {
			//todo: gui err
			err := open.Run(im.GetRealPath())
			if err != nil {
				panic(err)
			}
		}),
		fyne.NewMenuItem("Find Similar Images", func() {
			data, err := gui.db.ReadEmbedding(im.ID)
			// todo: GUI err
			if err != nil {
				panic(err)
			}
			basedirs := gui.getActiveBasedirsID()
			imgs, err := gui.db.MatchEmbeddingsWithFilter(data, QueryFilter{Limit: 64, BaseDirs: basedirs})
			if err != nil {
				panic(err)
			}
			gui.imageList.RemoveAll()
			gui.ShowImages(imgs)
		}),
	}
	menu := fyne.NewMenu("Image", items...)

	popup := widget.NewPopUpMenu(menu, gui.window.Canvas())
	popup.ShowAtPosition(pe.AbsolutePosition)
}

// todo: this should be a new object type
func (gui *GUI) buildSearchGUI() *fyne.Container {
	searchbox := widget.NewEntry()
	btn := widget.NewButton("Search", func() {
		gui.EmptyQuery()
		fmt.Println(gui.getActiveBasedirsString())
	})

	final := container.NewGridWithColumns(2, searchbox, btn)

	return final
}

// assembles the main gui window and wires all the components on it
func (gui *GUI) Build() {
	// LEFT ----------------------------------------------------
	indexesLabel := widget.NewLabel("Indexes")
	gui.rebuildBasedirs()
	// new basedir button & implementation
	basedirsWrapper := container.NewHScroll(gui.guiBasedirs)

	indexesButtons := gui.buildIndexButtons()

	imgInfoLabel := widget.NewLabel("Image Info")
	appLogLabel := widget.NewLabel("Log")

	imgInfo := widget.NewRichText()
	appLog := widget.NewRichText()
	appLog.AppendMarkdown("**Started**")
	leftContainer := container.NewVBox(
		indexesLabel, basedirsWrapper, indexesButtons,
		imgInfoLabel, imgInfo,
		appLogLabel, appLog,
	)
	// RIGHT ----------------------------------------------------
	searchbox := gui.buildSearchGUI()
	gui.imageList = NewImageList(gui.ShowThumbnailMenu)
	scroll := container.NewVScroll(gui.imageList)
	rightContainer := container.NewBorder(searchbox, nil, nil, nil, scroll)

	// DIALOGUES ---------------------------------------------------
	gui.indexingDialogue = NewImageProcessDialogue(gui.window)

	//split := widget.NewSeparator()
	total := container.NewBorder(nil, nil, leftContainer, nil, rightContainer)
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
			logBox.Append("Started\n")
			threadsToUse := max(runtime.NumCPU()-2, 2)
			aw := archivewalk.NewArchiveWalker(threadsToUse, errCh, true, true, ipd.processor.Handler)
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

// the query to use when no search text or image have been specified
func (gui *GUI) EmptyQuery() {
	basedirs := gui.getActiveBasedirsID()
	if len(basedirs) == 0 {
		return
	}
	imgs, err := gui.db.ReadImages(QueryFilter{Limit: 128, BaseDirs: basedirs}, OrderByAestheticDesc)
	if err != nil {
		panic(err)
	}
	gui.imageList.RemoveAll()
	gui.ShowImages(imgs)
}

// Update gui content to display the given images in order.
func (gui *GUI) ShowImages(dbImages []Image) {
	dbImages, err := gui.db.AugmentImages(dbImages)
	if err != nil {
		panic(err)
	}
	for _, img := range dbImages {
		//todo: is it threadsafe to add to imageList like this?
		//todo: preserve order
		go func() {
			imgImg, err := img.Load()
			imgImg = imageutil.ScaleImageRGBA(imgImg, THUMBNAIL_SIZE)
			if err != nil {
				panic(err)
			}
			gui.imageList.AddImage(imgImg, img)
		}()
	}
}
