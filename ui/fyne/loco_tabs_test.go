package fyneui

import (
	"errors"
	"image/png"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestInlineLocoName(t *testing.T) {
	v, s := setupView(t)
	v.tabs.SelectIndex(1)
	v.Window.Show()
	panel := v.panels[3]
	h := v.runTabs.headers[panel.tab]
	test.DoubleTap(h.title)
	if !h.editing || h.title.Visible() || !h.editor.Visible() || v.Window.Canvas().Focused() != h.editor || panel.setup != nil {
		t.Fatalf("inline state editing=%v title=%v editor=%v focus=%T setup=%v mobile=%v", h.editing, h.title.Visible(), h.editor.Visible(), v.Window.Canvas().Focused(), panel.setup != nil, h.title.mobile())
	}
	h.editor.SetText("Étoile 🚂")
	if h.editor.Size().Width < 180 || h.editor.MinSize().Width != 180 {
		t.Fatalf("inline editor too narrow: %v", h.editor.Size())
	}
	if path := os.Getenv("DCCEX_INLINE_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		encodeErr := png.Encode(file, v.Window.Canvas().Capture())
		closeErr := file.Close()
		if encodeErr != nil || closeErr != nil {
			t.Fatal(encodeErr, closeErr)
		}
	}
	// Incoming operational snapshots must not overwrite a draft.
	v.Render(v.last)
	if h.editor.Text != "Étoile 🚂" {
		t.Fatal("snapshot overwrote editor")
	}
	h.editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "Étoile 🚂" })
	if h.editing || panel.tab.Text != "Étoile 🚂" || !h.title.Visible() {
		t.Fatal("submitted name not shown")
	}
	test.DoubleTap(h.title)
	h.editor.SetText("cancel me")
	h.editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if h.editing || panel.tab.Text != "Étoile 🚂" {
		t.Fatal("Escape did not cancel")
	}
	test.DoubleTap(h.title)
	h.editor.SetText("")
	v.Window.Canvas().Focus(v.cab)
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "" })
	if h.editing || panel.tab.Text != "Loco 3" {
		t.Fatal("focus loss did not save blank fallback")
	}
	// Setup continues to use the same persisted name, without becoming inline.
	test.Tap(panel.setupButton)
	panel.name.SetText("From Setup")
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "From Setup" })
	panel.setup.Hide()
	test.DoubleTap(h.title)
	if h.editor.Text != "From Setup" {
		t.Fatal("inline editor did not load Setup's name")
	}
	h.finish(false)
}

func TestInlineNameTargetsItsOwnTab(t *testing.T) {
	v, s := setupView(t)
	if err := s.Post(func(c *th.Controller) error { return c.AddCab(7) }); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 2 })
	h := v.runTabs.headers[v.panels[3].tab]
	h.title.DoubleTapped(nil)
	h.editor.SetText("Only three")
	h.editor.OnSubmitted(h.editor.Text)
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "Only three" })
	if v.last.Throttles[1].Name != "" || v.runTabs.Selected() != v.panels[3].tab {
		t.Fatal("rename affected another tab")
	}
	if err := v.runTabs.rename(container.NewTabItem("gone", widget.NewLabel("")), "wrong"); err == nil {
		t.Fatal("removed tab accepted a rename")
	}
	if name := v.runTabs.name(container.NewTabItem("gone", widget.NewLabel(""))); name != "" {
		t.Fatal("removed tab returned another name")
	}
}

func TestLocoTabGestures(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		calls := 0
		button := &locoTabTitle{mobile: func() bool { return mobile }, edit: func() { calls++ }}
		button.DoubleTapped(nil)
		if (calls == 1) == mobile {
			t.Fatal("double-click must edit on desktop only")
		}
		button.TappedSecondary(nil)
		if calls != 1 {
			t.Fatal("mobile long hold must edit; desktop right-click must not")
		}
	}
}

func TestLocoTabsLifecycle(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("editable tabs")
	t.Cleanup(w.Close)
	one := container.NewTabItem("one", widget.NewLabel("one"))
	two := container.NewTabItem("two", widget.NewLabel("two"))
	var names []string
	tabs := newLocoTabs(w, func(item *container.TabItem) string { return item.Text }, func(_ *container.TabItem, name string) error {
		names = append(names, name)
		return nil
	})
	w.SetContent(tabs)
	w.Resize(fyne.NewSize(320, 200))
	w.Show()
	tabs.SetItems([]*container.TabItem{one, two})
	first, second := tabs.headers[one], tabs.headers[two]
	selected, unselected := 0, 0
	tabs.OnSelected = func(*container.TabItem) { selected++ }
	tabs.OnUnselected = func(*container.TabItem) { unselected++ }
	test.Tap(first.title)
	tabs.Select(nil)
	if selected != 0 || unselected != 0 {
		t.Fatal("same/absent selection fired callbacks")
	}
	first.begin()
	first.editor.SetText("draft")
	first.begin()
	if first.editor.Text != "draft" {
		t.Fatal("reopening active editor reset draft")
	}
	tabs.Select(two)
	if selected != 1 || unselected != 1 || len(names) != 1 || names[0] != "draft" || first.editing {
		t.Fatal("switching tabs failed to commit valid draft once")
	}
	closed := false
	tabs.CloseIntercept = func(item *container.TabItem) { closed = item == two }
	// The close target remains separate from the editable title.
	closeButton := second.root.Objects[1].(*widget.Button)
	test.Tap(closeButton)
	if !closed {
		t.Fatal("close interception lost")
	}
	tabs.CloseIntercept = nil
	test.Tap(closeButton)
	second.begin()
	second.editor.SetText("discard on removal")
	tabs.SetItems([]*container.TabItem{one})
	if second.editing || len(tabs.headers) != 1 || tabs.Selected() != one || len(names) != 1 {
		t.Fatal("removal saved orphan draft or retained old header")
	}
	second.begin()
	if second.editing {
		t.Fatal("removed tab reopened")
	}
	tabs.SetItems(nil)
	if tabs.Selected() != nil || len(tabs.content.Objects) != 0 {
		t.Fatal("empty tab set retained content")
	}
	// Empty selection and nil callbacks are valid during snapshot restoration.
	tabs.Items = []*container.TabItem{one}
	tabs.OnSelected, tabs.OnUnselected = nil, nil
	tabs.Select(one)
	tabs.Items = append(tabs.Items, two)
	tabs.Select(two)
	test.Tap(tabs.all)
	menu := w.Canvas().Focused().(*widget.PopUpMenu)
	test.Tap(menu.Items[0].(fyne.Tappable))
	if tabs.Selected() != one {
		t.Fatal("overflow menu did not select tab")
	}
	menu.Hide()
}

func TestInlineNameValidationAndSaveFailure(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("validation")
	t.Cleanup(w.Close)
	item := container.NewTabItem("one", widget.NewLabel("one"))
	saves := 0
	tabs := newLocoTabs(w, func(*container.TabItem) string { return "" }, func(*container.TabItem, string) error {
		saves++
		return errors.New("session closed")
	})
	w.SetContent(tabs)
	tabs.SetItems([]*container.TabItem{item})
	h := tabs.headers[item]
	h.begin()
	for _, invalid := range []string{"bad\nname", strings.Repeat("x", 81)} {
		h.editor.SetText(invalid)
		h.editor.OnSubmitted(invalid)
		if !h.editing || h.editor.Validate() == nil || saves != 0 {
			t.Fatal("invalid name accepted")
		}
	}
	h.editor.SetText("valid")
	h.editor.OnSubmitted("valid")
	if saves != 1 || !h.editing {
		t.Fatal("failed save discarded editor")
	}
	h.finish(false)
	if h.editing {
		t.Fatal("failed save cannot be cancelled")
	}
}
