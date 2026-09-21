package fyneui

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestTabFlow(t *testing.T) {
	test.NewTempApp(t)
	var objects []fyne.CanvasObject
	for i := 0; i < 4; i++ {
		r := canvas.NewRectangle(color.White)
		r.SetMinSize(fyne.NewSize(100, 40))
		objects = append(objects, r)
	}
	objects[3].Hide()
	if height := flowTabHeaders(objects, 210); height <= 40 {
		t.Fatal("no second row", height)
	}
	if objects[0].Position().Y != objects[1].Position().Y || objects[2].Position().Y <= objects[1].Position().Y {
		t.Fatal("wrong wrap")
	}
	if height := flowTabHeaders(objects, 400); height != 40 {
		t.Fatal("landscape did not collapse rows", height)
	}
	flowTabHeaders(objects, 80)
	for _, object := range objects[:3] {
		if object.Size().Width != 80 {
			t.Fatal("overflow")
		}
	}
	flowTabHeaders(objects, 0)
	if height := flowTabHeaders(nil, 360); height != 0 {
		t.Fatal(height)
	}
}

func TestWrappedLocoTabs(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("wrapping")
	defer w.Close()
	names := map[*container.TabItem]string{}
	renames := 0
	tabs := newLocoTabs(w, func(item *container.TabItem) string { return names[item] }, func(item *container.TabItem, name string) error {
		names[item] = name
		item.Text = name
		renames++
		return nil
	}, true)
	var items []*container.TabItem
	for i := 0; i < 14; i++ {
		item := container.NewTabItem("Loco", widget.NewLabel("Throttle"))
		items = append(items, item)
	}
	items[1].Text = strings.Repeat("🚂", 40)
	names[items[1]] = items[1].Text
	tabs.SetItems(items)
	w.SetContent(tabs)
	w.Resize(fyne.NewSize(360, 600))
	w.Show()
	selections := 0
	tabs.OnSelected = func(*container.TabItem) { selections++ }
	tabs.Resize(fyne.NewSize(360, 600))
	narrowY := tabs.headers[items[13]].root.Position().Y
	if narrowY == 0 {
		t.Fatal("tabs did not wrap")
	}
	for _, h := range tabs.headers {
		if h.root.Position().X+h.root.Size().Width > 360 {
			t.Fatal("tab exceeds screen", h.root.Position(), h.root.Size())
		}
	}
	tabs.Resize(fyne.NewSize(800, 360))
	if tabs.headers[items[13]].root.Position().Y >= narrowY {
		t.Fatal("rotation did not reflow")
	}
	if selections != 0 {
		t.Fatal("layout changed active locomotive")
	}
	tabs.Resize(fyne.NewSize(360, 300))
	tabs.Select(items[13])
	if tabs.scroll.Offset.Y <= 0 {
		t.Fatalf("selected last tab not revealed: scroll=%v content=%v min=%v last=%v tabs=%v", tabs.scroll.Size(), tabs.bar.Size(), tabs.bar.MinSize(), tabs.headers[items[13]].root.Position(), tabs.Size())
	}
	tabs.Select(items[0])
	if tabs.scroll.Offset.Y != 0 {
		t.Fatal("first tab not revealed")
	}
	if selections != 2 {
		t.Fatal(selections)
	}
	h := tabs.headers[items[1]]
	if h.title.Text == items[1].Text || h.title.AccessibilityLabel() != items[1].Text {
		t.Fatal("long name not safely shortened")
	}
	h.title.TappedSecondary(nil)
	if !h.editing || tabs.Selected() != items[0] || selections != 2 {
		t.Fatal("rename switched locomotive")
	}
	if h.editor.Text != names[items[1]] {
		t.Fatal("editor lost full name")
	}
	h.editor.SetText("Shunter")
	h.editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if renames != 1 || h.title.Text != "Shunter" {
		t.Fatal("rename not saved")
	}
	test.Tap(h.title)
	if tabs.Selected() != items[1] || selections != 3 {
		t.Fatal("tap not immediate")
	}
	tabs.SetItems(items[:1])
	if len(tabs.headers) != 1 || tabs.Selected() != items[0] {
		t.Fatal("removed headers retained")
	}
	renderer := test.WidgetRenderer(tabs)
	if renderer.MinSize().Width > 360 || len(renderer.Objects()) != 3 {
		t.Fatal("invalid mobile renderer")
	}
	renderer.Destroy()
}
