package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

var directionPurple = color.NRGBA{R: 112, G: 66, B: 176, A: 255}

// DirectionSwitch is a full-width segmented slider, not an action-labelled
// button. Dragging previews movement; only release commits a direction.
type DirectionSwitch struct {
	widget.DisableableWidget
	Selected          string
	OnChanged         func(string)
	position          float32
	dragging, focused bool
}

func newDirectionSwitch(changed func(string)) *DirectionSwitch {
	s := &DirectionSwitch{Selected: "Fwd", position: 1, OnChanged: changed}
	s.ExtendBaseWidget(s)
	return s
}
func (s *DirectionSwitch) SetSelected(value string) {
	if value != "Rev" && value != "Fwd" {
		return
	}
	s.Selected = value
	if !s.dragging {
		if value == "Fwd" {
			s.position = 1
		} else {
			s.position = 0
		}
	}
	s.Refresh()
}
func (s *DirectionSwitch) choose(forward bool) {
	if s.Disabled() {
		return
	}
	next := "Rev"
	if forward {
		next = "Fwd"
	}
	previous := s.Selected
	s.SetSelected(next)
	if previous != next && s.OnChanged != nil {
		s.OnChanged(next)
	}
}
func (s *DirectionSwitch) Tapped(e *fyne.PointEvent) {
	if s.Disabled() {
		return
	}
	if c := fyne.CurrentApp().Driver().CanvasForObject(s); c != nil {
		c.Focus(s)
	}
	s.choose(e.Position.X >= s.Size().Width/2)
}
func (s *DirectionSwitch) Dragged(e *fyne.DragEvent) {
	if s.Disabled() || s.Size().Width <= 0 {
		return
	}
	s.dragging = true
	s.position = (e.Position.X - s.Size().Width/4) / (s.Size().Width / 2)
	if s.position < 0 {
		s.position = 0
	}
	if s.position > 1 {
		s.position = 1
	}
	s.Refresh()
}
func (s *DirectionSwitch) DragEnd() {
	if !s.dragging {
		return
	}
	s.dragging = false
	if s.Disabled() {
		s.SetSelected(s.Selected)
		return
	}
	s.choose(s.position >= 0.5)
}
func (s *DirectionSwitch) Disable() {
	s.dragging = false
	s.SetSelected(s.Selected)
	s.DisableableWidget.Disable()
}
func (s *DirectionSwitch) FocusGained()   { s.focused = true; s.Refresh() }
func (s *DirectionSwitch) FocusLost()     { s.focused = false; s.Refresh() }
func (s *DirectionSwitch) TypedRune(rune) {}
func (s *DirectionSwitch) TypedKey(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyLeft:
		s.choose(false)
	case fyne.KeyRight:
		s.choose(true)
	case fyne.KeySpace:
		s.choose(s.Selected != "Fwd")
	}
}
func (s *DirectionSwitch) CreateRenderer() fyne.WidgetRenderer {
	r := &directionRenderer{s: s, track: canvas.NewRectangle(color.Transparent), thumb: canvas.NewRectangle(color.Transparent), rev: canvas.NewText("Rev", color.White), fwd: canvas.NewText("Fwd", color.White)}
	r.rev.Alignment = fyne.TextAlignCenter
	r.fwd.Alignment = fyne.TextAlignCenter
	r.Refresh()
	return r
}

type directionRenderer struct {
	s            *DirectionSwitch
	track, thumb *canvas.Rectangle
	rev, fwd     *canvas.Text
}

func (r *directionRenderer) MinSize() fyne.Size { return fyne.NewSize(120, 36) }
func (r *directionRenderer) Layout(size fyne.Size) {
	pad := float32(3)
	height := size.Height - 2*pad
	width := (size.Width - 2*pad) / 2
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	r.track.Resize(size)
	r.track.CornerRadius = size.Height / 2
	r.thumb.Move(fyne.NewPos(pad+width*r.s.position, pad))
	r.thumb.Resize(fyne.NewSize(width, height))
	r.thumb.CornerRadius = height / 2
	textHeight := r.rev.MinSize().Height
	r.rev.Move(fyne.NewPos(pad, (size.Height-textHeight)/2))
	r.rev.Resize(fyne.NewSize(width, textHeight))
	r.fwd.Move(fyne.NewPos(pad+width, (size.Height-textHeight)/2))
	r.fwd.Resize(fyne.NewSize(width, textHeight))
}
func (r *directionRenderer) Refresh() {
	r.track.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.track.StrokeWidth = 1
	r.track.StrokeColor = theme.Color(theme.ColorNameInputBorder)
	if r.s.focused {
		r.track.StrokeColor = theme.Color(theme.ColorNameFocus)
	}
	r.thumb.FillColor = directionPurple
	r.rev.Color = theme.Color(theme.ColorNameForeground)
	r.fwd.Color = r.rev.Color
	if r.s.Disabled() {
		r.thumb.FillColor = theme.Color(theme.ColorNameDisabledButton)
		r.rev.Color = theme.Color(theme.ColorNameDisabled)
		r.fwd.Color = r.rev.Color
	}
	r.rev.TextStyle.Bold = r.s.Selected == "Rev"
	r.fwd.TextStyle.Bold = r.s.Selected == "Fwd"
	if !r.s.Disabled() {
		if r.s.Selected == "Rev" {
			r.rev.Color = color.White
		} else {
			r.fwd.Color = color.White
		}
	}
	r.Layout(r.s.Size())
	r.track.Refresh()
	r.thumb.Refresh()
	r.rev.Refresh()
	r.fwd.Refresh()
}
func (r *directionRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.thumb, r.rev, r.fwd}
}
func (*directionRenderer) Destroy() {}
