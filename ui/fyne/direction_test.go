package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"image/color"
	"testing"
)

func TestDirectionSwitchTapDragKeyboardAndSizing(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("direction")
	defer w.Close()
	var changes []string
	s := newDirectionSwitch(func(value string) { changes = append(changes, value) })
	w.SetContent(s)
	s.Resize(fyne.NewSize(240, 40))
	renderer := test.WidgetRenderer(s).(*directionRenderer)
	if renderer.thumb.FillColor != directionPurple || renderer.fwd.Color != color.White {
		t.Fatal("forward must use purple with white selected text")
	}
	test.TapAt(s, fyne.NewPos(20, 20))
	if s.Selected != "Rev" || len(changes) != 1 {
		t.Fatal("tap reverse")
	}
	if renderer.thumb.FillColor != directionPurple || renderer.rev.Color != color.White {
		t.Fatal("reverse must use the same purple with white selected text")
	}
	test.TapAt(s, fyne.NewPos(20, 20))
	if len(changes) != 1 {
		t.Fatal("reselection toggled")
	}
	s.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(230, 20)}})
	if s.Selected != "Rev" || len(changes) != 1 {
		t.Fatal("drag changed direction before release")
	}
	s.DragEnd()
	if s.Selected != "Fwd" || len(changes) != 2 {
		t.Fatal("drag did not commit")
	}
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if s.Selected != "Rev" {
		t.Fatal("left arrow")
	}
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if s.Selected != "Fwd" {
		t.Fatal("right arrow")
	}
	s.SetSelected("Rev")
	before := len(changes)
	if before != 4 {
		t.Fatal("render echoed command")
	}
	s.Disable()
	test.TapAt(s, fyne.NewPos(230, 20))
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	s.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(230, 20)}})
	s.DragEnd()
	if len(changes) != before || s.Selected != "Rev" {
		t.Fatal("disabled interaction")
	}
	r := test.WidgetRenderer(s)
	dr := r.(*directionRenderer)
	if dr.thumb.FillColor == dr.rev.Color {
		t.Fatal("disabled selected label has no contrast")
	}
	if dr.thumb.FillColor == directionPurple {
		t.Fatal("disabled selector must remain neutral")
	}
	for _, size := range []fyne.Size{fyne.NewSize(120, 36), fyne.NewSize(240, 40), fyne.NewSize(300, 48)} {
		s.Resize(size)
		r.Layout(size)
		if r.Objects()[0].Size() != size {
			t.Fatal("track did not fill control slot")
		}
		for _, object := range r.Objects() {
			if object.Position().X < 0 || object.Position().Y < 0 || object.Position().X+object.Size().Width > size.Width+0.1 || object.Position().Y+object.Size().Height > size.Height+0.1 {
				t.Fatal("selector overflow", size, object.Position(), object.Size())
			}
		}
	}
}
