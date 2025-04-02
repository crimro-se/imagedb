package main

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ImageList struct {
	*fyne.Container
	callback func(*fyne.PointEvent, Image)
}

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

func (il *ImageList) Clear() {
	// Dispose all children first
	for _, obj := range il.Objects {
		if ib, ok := obj.(interface{ Dispose() }); ok {
			ib.Dispose()
		}
	}
	// Then remove them
	il.Objects = nil
	il.Refresh()
}

// image button

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

func (ib *ImageButtonWithData[T]) Dispose() {
	// Clear the image resource
	ib.Image.Image = nil
	ib.Image = nil
}
