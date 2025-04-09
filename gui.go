package main

import (
	"context"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/crimro-se/imagedb/archivewalk"
	"github.com/crimro-se/imagedb/embeddingserver"
	"github.com/crimro-se/imagedb/imageutil"
	"github.com/crimro-se/imagedb/internal/imagedbutil"
	"github.com/skratchdot/open-golang/open"
)

type GUI struct {
	window   fyne.Window
	db       *Database
	conf     *Config
	actables []*widget.DisableableWidget

	guiBasedirs   *fyne.Container //vbox container
	basedirsState map[int64]binding.Bool

	imageList *ImageList
	log       *widget.Entry
	imgInfo   *widget.Entry

	indexingDialogue *ImageProcessDialogue
	busyDialogue     *BusyDialogue
	active           bool
}

func NewGUI(window fyne.Window, db *Database, conf *Config) *GUI {
	gui := GUI{
		window:   window,
		db:       db,
		conf:     conf,
		actables: make([]*widget.DisableableWidget, 0),

		basedirsState: make(map[int64]binding.Bool),
		guiBasedirs:   container.NewVBox(),
	}
	gui.Build()
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
		gui.ShowError(err)
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
		check := widget.NewCheckWithData(imagedbutil.MidTruncateString(basedir.Directory, 36), oldstate)
		gui.basedirsState[basedir.ID] = oldstate
		gui.guiBasedirs.Add(check)
	}
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
			gui.ShowError(err)
		}

	}
	return ints
}

// list of Basedir that the user has enabled in the UI
func (gui *GUI) getActiveBasedirs() []Basedir {
	filteredBasedirs := make([]Basedir, 0)
	activeIDs := gui.getActiveBasedirsID()
	allBasedirs, err := gui.db.GetAllBasedir()
	if err != nil {
		gui.ShowError(err)
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
		err := gui.indexingDialogue.Show("db.sqlite", activeBasedirs[0], gui.conf.API_SERVER)
		if err != nil {
			gui.ShowError(err)
			return
		}
	})

	deleteIndexBtn := widget.NewButton("Delete", func() {
		activeBasedirs := gui.getActiveBasedirs()
		if len(activeBasedirs) != 1 {
			dialog.NewInformation("", "Select only exactly one index first", gui.window).Show()
			return
		}

		gui.deactivateAll()
		defer gui.activateAll()
		err := gui.db.DeleteBasedir(activeBasedirs[0].ID)
		if err != nil {
			dialog.NewError(err, gui.window).Show()
		}
		gui.rebuildBasedirs()
	})

	gui.actables = append(gui.actables, &addIndexBtn.DisableableWidget)
	gui.actables = append(gui.actables, &updateIndexBtn.DisableableWidget)
	gui.actables = append(gui.actables, &deleteIndexBtn.DisableableWidget)
	indexesButtons := container.NewHBox(addIndexBtn, updateIndexBtn, deleteIndexBtn)
	padded := container.New(layout.NewCustomPaddedLayout(0, 0, 48, 48), indexesButtons)
	return padded
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

// todo: this should be a new object type
func (gui *GUI) buildSearchGUI() *fyne.Container {
	searchbox := widget.NewEntry()
	btn := widget.NewButton("Search", func() {
		if len(searchbox.Text) == 0 {
			gui.QueryNone()
			return
		}
		gui.QueryText(searchbox.Text, gui.conf.API_SERVER)
	})

	final := container.NewGridWithColumns(2, searchbox, btn)

	return final
}

// generates a queryfilter based on the GUI's current settings
// todo: gui interface for more settings
func (gui *GUI) getQueryFilter() QueryFilter {
	return QueryFilter{BaseDirs: gui.getActiveBasedirsID(), Limit: gui.conf.QUERY_RESULTS}
}

func (gui *GUI) QueryText(query string, server string) {
	client := embeddingserver.NewClient(server)
	embedding, err := client.GetTextEmbedding(query)
	if err != nil {
		gui.ShowError(err)
		return
	}
	embeddingBytes, err := sqlite_vec.SerializeFloat32(embedding.Embedding)
	if err != nil {
		gui.ShowError(err)
		return
	}
	gui.QueryImages(embeddingBytes)
}

// Finds and displays images in the database that are most similar to the provided embedding data.
// see also: sqlite_vec.SerializeFloat32
func (gui *GUI) QueryImages(embedding []byte) {
	imgs, err := gui.db.MatchEmbeddingsWithFilter(embedding, gui.getQueryFilter())
	if err != nil {
		gui.ShowError(err)
		return
	}
	gui.ShowImages(imgs)
}

// the query to use when no search text or image have been specified
func (gui *GUI) QueryNone() {
	basedirs := gui.getActiveBasedirsID()
	if len(basedirs) == 0 {
		gui.ShowError(fmt.Errorf("no active basedirs to query"))
		return
	}
	imgs, err := gui.db.ReadImages(QueryFilter{Limit: 128, BaseDirs: basedirs}, OrderByAestheticDesc)
	if err != nil {
		gui.ShowError(err)
		return
	}
	gui.ShowImages(imgs)
}

func (gui *GUI) ShowError(err error) {
	gui.log.Append(err.Error() + "\n")
}

func (gui *GUI) ShowImageDetails(img Image) {
	var sb strings.Builder
	sb.WriteString(img.GetRealPath())
	sb.WriteString("\n w: ")
	sb.WriteString(strconv.Itoa(int(img.Width)))
	sb.WriteString("  h: ")
	sb.WriteString(strconv.Itoa(int(img.Height)))
	sb.WriteString("\n Aesthetic: ")
	sb.WriteString(fmt.Sprintf("%v \n", img.Aesthetic.Float64))
	gui.imgInfo.Text = sb.String()
	gui.imgInfo.Refresh()
}

func (gui *GUI) ShowImages(dbImages []Image) {
	gui.busyDialogue.Show("Resizing images...")
	defer gui.busyDialogue.Hide()
	gui.imageList.Clear()

	type resultData struct {
		index   int
		img     image.Image
		imgData Image
		err     error
	}
	type jobData struct {
		index int
		img   Image
	}

	// Augment images
	augmentedImages, err := gui.db.AugmentImages(dbImages)
	if err != nil {
		gui.ShowError(fmt.Errorf("error augmenting images: %v", err))
		return
	}

	// Create channels for jobs and results
	jobs := make(chan jobData, len(augmentedImages))
	results := make(chan resultData, len(augmentedImages))

	var wg sync.WaitGroup

	// Start worker pool
	for range gui.conf.THREADS_FOR_THUMBNAILS {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Load and scale the image
				imgImg, err := job.img.Load()
				if err != nil {
					results <- resultData{index: job.index, err: err}
					continue
				}

				scaledImg := imageutil.ScaleImageRGBA(imgImg, gui.conf.IMAGE_SIZE_THUMBNAIL)
				imgImg = nil // Ensure release

				results <- resultData{
					index:   job.index,
					img:     scaledImg,
					imgData: job.img,
					err:     nil,
				}
			}
		}()
	}

	// Send jobs to workers
	for i, img := range augmentedImages {
		jobs <- jobData{index: i, img: img}
	}
	close(jobs)

	// Close the results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	orderedResults := make([]resultData, len(augmentedImages))

	for result := range results {
		if result.err != nil {
			gui.ShowError(fmt.Errorf("error loading image %d: %v", result.index, result.err))
		}
		orderedResults[result.index] = resultData{img: result.img, imgData: result.imgData, err: result.err}
	}

	// Add images to the list in order
	for _, result := range orderedResults {
		if result.err != nil || result.img == nil {
			continue // Skip failed images. error already displayed earlier.
		}
		gui.imageList.AddImage(result.img, result.imgData)
	}
	gui.imageList.Refresh()
}

/* Display a pop-up menu when an image is clicked (for now, I don't care which mouse button clicked.)
 */
func (gui *GUI) ShowThumbnailMenu(pe *fyne.PointEvent, im Image) {

	gui.ShowImageDetails(im)

	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Open Image", func() {
			err := open.Run(im.GetRealPath())
			if err != nil {
				gui.ShowError(err)
				return
			}
		}),
		fyne.NewMenuItem("Find Similar Images", func() {
			data, err := gui.db.ReadEmbedding(im.ID)
			if err != nil {
				gui.ShowError(err)
				return
			}
			gui.QueryImages(data)
		}),
	}
	menu := fyne.NewMenu("Image", items...)

	popup := widget.NewPopUpMenu(menu, gui.window.Canvas())
	popup.ShowAtPosition(pe.AbsolutePosition)
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

	gui.imgInfo = widget.NewMultiLineEntry()
	gui.log = widget.NewMultiLineEntry()
	gui.log.Append("Started\n")
	split := widget.NewSeparator()
	leftContainer := container.NewVBox(
		indexesLabel, basedirsWrapper, indexesButtons, split,
		imgInfoLabel, gui.imgInfo,
		appLogLabel, gui.log,
	)
	// RIGHT ----------------------------------------------------
	searchbox := gui.buildSearchGUI()
	gui.imageList = NewImageList(gui.ShowThumbnailMenu, gui.conf.IMAGE_SIZE_THUMBNAIL)
	scroll := container.NewVScroll(gui.imageList)
	rightContainer := container.NewBorder(searchbox, nil, nil, nil, scroll)

	// DIALOGUES ---------------------------------------------------
	gui.busyDialogue = NewBusyDialogue(gui.window)
	gui.indexingDialogue = NewImageProcessDialogue(gui.window, gui.conf.THREADS_FOR_INDEXING)

	total := container.NewBorder(nil, nil, leftContainer, nil, rightContainer)
	gui.window.SetContent(total)
	gui.window.Resize(fyne.NewSquareSize(900))
}

type BusyDialogue struct {
	*dialog.CustomDialog
	label    *widget.Label
	activity *widget.Activity
}

func NewBusyDialogue(w fyne.Window) *BusyDialogue {
	content := container.NewVBox()
	bd := &BusyDialogue{
		CustomDialog: dialog.NewCustomWithoutButtons("Busy", content, w),
		label:        widget.NewLabel(""),
		activity:     widget.NewActivity(),
	}
	content.Add(bd.label)
	content.Add(bd.activity)
	return bd
}

func (bd *BusyDialogue) Show(text string) {
	bd.label.SetText(text)
	bd.activity.Start()
	bd.CustomDialog.Show()
}

func (bd *BusyDialogue) Hide() {
	bd.activity.Stop()
	bd.CustomDialog.Hide()
}

type ImageProcessDialogue struct {
	*dialog.CustomDialog
	threads       int
	processor     *ImageProcessor
	displayedPath binding.String
	basedir       Basedir
	ctx           context.Context
	ctxCancel     context.CancelFunc
}

// prepares the dialogue for use then shows it
func (ipd *ImageProcessDialogue) Show(dbfile string, basedir Basedir, apiserver string) error {
	var err error
	ipd.basedir = basedir
	ipd.displayedPath.Set(basedir.Directory)
	ipd.processor, err = NewImageProcessor(dbfile, basedir, apiserver)
	if err != nil {
		return err
	}
	ipd.ctx, ipd.ctxCancel = context.WithCancel(context.Background())
	ipd.CustomDialog.Show()
	return nil
}

// a dialogue that handles directory walking and indexing
func NewImageProcessDialogue(w fyne.Window, threads int) *ImageProcessDialogue {
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
			aw := archivewalk.NewArchiveWalker(threads, errCh, true, true, ipd.processor.Handler)
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
