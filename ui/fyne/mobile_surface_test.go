package fyneui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestMobileSurfaceUsesCurrentTheme(t *testing.T) {
	a := test.NewTempApp(t)
	content := widget.NewLabel("Readable text")
	surface := newMobileSurface(content)
	r := test.WidgetRenderer(surface).(*mobileSurfaceRenderer)
	for _, selected := range []fyne.Theme{theme.LightTheme(), theme.DarkTheme(), theme.LightTheme()} {
		a.Settings().SetTheme(selected)
		surface.Refresh()
		want := theme.Color(theme.ColorNameBackground)
		if r.background.FillColor != want {
			t.Fatalf("background = %v, want %v", r.background.FillColor, want)
		}
		_, _, _, alpha := r.background.FillColor.RGBA()
		if alpha != 0xffff {
			t.Fatal("background must be opaque")
		}
	}
	size := fyne.NewSize(360, 780)
	surface.Resize(size)
	if r.background.Size() != size || content.Size() != size {
		t.Fatalf("surface did not fill screen: background %v, content %v", r.background.Size(), content.Size())
	}
	if surface.MinSize() != content.MinSize() {
		t.Fatal("background changed content minimum size")
	}
}
