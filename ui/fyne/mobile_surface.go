package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// mobileSurface paints an opaque themed background instead of relying on the
// native surface clear color. Text and background must use the same theme even
// when the Android rendering surface underneath the application is black.
type mobileSurface struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func newMobileSurface(content fyne.CanvasObject) *mobileSurface {
	s := &mobileSurface{content: content}
	s.ExtendBaseWidget(s)
	return s
}

func (s *mobileSurface) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(s.Theme().Color(theme.ColorNameBackground, fyne.CurrentApp().Settings().ThemeVariant()))
	return &mobileSurfaceRenderer{
		WidgetRenderer: widget.NewSimpleRenderer(container.NewStack(background, s.content)),
		surface:        s,
		background:     background,
	}
}

type mobileSurfaceRenderer struct {
	fyne.WidgetRenderer
	surface    *mobileSurface
	background *canvas.Rectangle
}

func (r *mobileSurfaceRenderer) Refresh() {
	r.background.FillColor = r.surface.Theme().Color(theme.ColorNameBackground, fyne.CurrentApp().Settings().ThemeVariant())
	r.WidgetRenderer.Refresh()
}
