package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// buttonRows reserves two equal-width cells per row, including odd final rows.
// Callers keep labels, input fields and other controls outside these rows.
func buttonRows(buttons ...fyne.CanvasObject) *fyne.Container {
	return container.NewGridWithColumns(2, buttons...)
}

func wrappedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

func (v *View) stationControlsMobile() fyne.CanvasObject {
	return container.NewVBox(widget.NewLabel("Station"), v.savedStations, buttonRows(v.saveStation, v.deleteStation))
}
