package fyneui

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
)

// locoTabs owns its headers because DocTabs does not expose editable titles or
// title gestures. Tab content and selection callbacks keep their existing API.
type locoTabs struct {
	widget.BaseWidget
	root                     *fyne.Container
	Items                    []*container.TabItem
	OnSelected, OnUnselected func(*container.TabItem)
	CloseIntercept           func(*container.TabItem)
	selected                 *container.TabItem
	headers                  map[*container.TabItem]*locoTabHeader
	bar, content             *fyne.Container
	scroll                   *container.Scroll
	all                      *widget.Button
	window                   fyne.Window
	name                     func(*container.TabItem) string
	rename                   func(*container.TabItem, string) error
	timing                   *tabTiming
	cab                      func(*container.TabItem) int
}

func newLocoTabs(window fyne.Window, name func(*container.TabItem) string, rename func(*container.TabItem, string) error) *locoTabs {
	t := &locoTabs{window: window, name: name, rename: rename, headers: make(map[*container.TabItem]*locoTabHeader)}
	t.bar = container.NewHBox()
	t.content = container.NewStack()
	t.scroll = container.NewHScroll(t.bar)
	t.all = widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() {
		items := make([]*fyne.MenuItem, 0, len(t.Items))
		for _, item := range t.Items {
			choice := fyne.NewMenuItem(item.Text, func() { t.Select(item) })
			choice.Checked = item == t.selected
			items = append(items, choice)
		}
		menu := widget.NewPopUpMenu(fyne.NewMenu("Locomotives", items...), window.Canvas())
		menu.ShowAtRelativePosition(fyne.NewPos(0, t.all.Size().Height), t.all)
	})
	t.all.Importance = widget.LowImportance
	t.root = container.NewBorder(container.NewVBox(container.NewBorder(nil, nil, nil, t.all, t.scroll), widget.NewSeparator()), nil, nil, nil, t.content)
	t.ExtendBaseWidget(t)
	return t
}

func (t *locoTabs) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.root) }

func (t *locoTabs) Selected() *container.TabItem { return t.selected }

func (t *locoTabs) Select(item *container.TabItem) {
	var timing *tabTimingSpan
	if t.timing != nil && t.cab != nil {
		timing = t.timing.span(t.cab(item), false)
	}
	timing.mark("selection_begin")
	if item == t.selected || !slices.Contains(t.Items, item) {
		timing.mark("selection_unchanged")
		return
	}
	old := t.selected
	if header := t.headers[old]; header != nil {
		header.finish(true)
	}
	t.selected = item
	if old != nil && t.OnUnselected != nil {
		t.OnUnselected(old)
	}
	t.Refresh()
	t.scroll.ScrollToOffset(t.headers[item].root.Position())
	timing.mark("selection_layout_complete")
	if t.OnSelected != nil {
		t.OnSelected(item)
	}
	timing.mark("selection_callback_complete")
}

func (t *locoTabs) SetItems(items []*container.TabItem) {
	t.Items = items
	for item, header := range t.headers {
		if !slices.Contains(items, item) {
			// A removed/reassigned address must not receive an editor's draft.
			header.finish(false)
			delete(t.headers, item)
		}
	}
	if !slices.Contains(items, t.selected) {
		t.selected = nil
		if len(items) > 0 {
			t.selected = items[0]
		}
	}
	t.Refresh()
}

func (t *locoTabs) Refresh() {
	buttons := make([]fyne.CanvasObject, 0, len(t.Items))
	for _, item := range t.Items {
		h := t.headers[item]
		if h == nil {
			h = newLocoTabHeader(t, item)
			t.headers[item] = h
		}
		h.title.SetText(item.Text)
		h.title.Importance = widget.LowImportance
		if item == t.selected {
			h.title.Importance = widget.MediumImportance
		}
		h.title.Refresh()
		buttons = append(buttons, h.root)
	}
	t.bar.Objects = buttons
	t.bar.Refresh()
	t.content.Objects = nil
	if t.selected != nil {
		t.content.Objects = []fyne.CanvasObject{t.selected.Content}
	}
	t.content.Refresh()
	t.BaseWidget.Refresh()
}

type locoTabHeader struct {
	tabs    *locoTabs
	item    *container.TabItem
	root    *fyne.Container
	title   *locoTabTitle
	editor  *locoNameEntry
	editing bool
}

func newLocoTabHeader(t *locoTabs, item *container.TabItem) *locoTabHeader {
	h := &locoTabHeader{tabs: t, item: item}
	h.title = &locoTabTitle{edit: h.begin, mobile: func() bool { return fyne.CurrentDevice().IsMobile() }}
	h.title.Text = item.Text
	h.title.input = func(stage string) {
		if t.timing != nil && t.cab != nil {
			t.timing.input(t.cab(item), stage)
		}
	}
	h.title.OnTapped = func() { t.Select(item) }
	h.title.ExtendBaseWidget(h.title)
	h.editor = &locoNameEntry{finish: h.finish}
	h.editor.ExtendBaseWidget(h.editor)
	h.editor.Validator = config.ValidateDisplayName
	h.editor.SetPlaceHolder("Loco <address>")
	h.editor.OnSubmitted = func(string) { h.finish(true) }
	h.editor.Hide()
	closeButton := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		if t.CloseIntercept != nil {
			t.CloseIntercept(item)
		}
	})
	closeButton.Importance = widget.LowImportance
	h.root = container.NewBorder(nil, nil, nil, closeButton, container.NewStack(h.title, h.editor))
	return h
}

func (h *locoTabHeader) begin() {
	if !slices.Contains(h.tabs.Items, h.item) {
		return
	}
	if !h.editing {
		h.editor.SetText(h.tabs.name(h.item))
		h.editing = true
		h.title.Hide()
		h.editor.Show()
		h.tabs.Refresh()
	}
	h.tabs.window.Canvas().Focus(h.editor)
	h.editor.TypedShortcut(&fyne.ShortcutSelectAll{})
}

func (h *locoTabHeader) finish(save bool) {
	if !h.editing {
		return
	}
	if save {
		if h.editor.Validate() != nil {
			return
		}
		if err := h.tabs.rename(h.item, h.editor.Text); err != nil {
			h.editor.SetValidationError(err)
			return
		}
	}
	h.editing = false
	if h.tabs.window.Canvas().Focused() == h.editor {
		h.tabs.window.Canvas().Unfocus()
	}
	h.editor.Hide()
	h.title.Show()
	h.tabs.Refresh()
}

type locoTabTitle struct {
	widget.Button
	edit        func()
	mobile      func() bool
	input       func(string)
	renameClick bool
}

func (b *locoTabTitle) noteInput(stage string) {
	if b.input != nil {
		b.input(stage)
	}
}

// Capture Ctrl on press, but act only on a completed click. This widget must not
// implement fyne.DoubleTappable: that would delay every ordinary tab selection.
func (b *locoTabTitle) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.renameClick = e.Modifier&fyne.KeyModifierControl != 0
		b.noteInput("pointer_down")
	}
}

func (b *locoTabTitle) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.noteInput("pointer_up")
	}
}

func (b *locoTabTitle) Tapped(e *fyne.PointEvent) {
	rename := b.renameClick
	b.renameClick = false
	if rename && !b.mobile() {
		b.noteInput("ctrl_click_dispatched")
		b.edit()
		return
	}
	b.noteInput("tap_dispatched")
	b.Button.Tapped(e)
}

// Fyne's mobile driver delivers a stationary long hold as a secondary tap.
// Desktop right-click remains unchanged; Ctrl-click edits there.
func (b *locoTabTitle) TappedSecondary(*fyne.PointEvent) {
	if b.mobile() {
		b.noteInput("long_hold_dispatched")
		b.edit()
	}
}

type locoNameEntry struct {
	widget.Entry
	finish func(bool)
}

func (e *locoNameEntry) MinSize() fyne.Size {
	size := e.Entry.MinSize()
	size.Width = 180
	return size
}

func (e *locoNameEntry) FocusLost() {
	e.Entry.FocusLost()
	e.finish(true)
}

func (e *locoNameEntry) TypedKey(event *fyne.KeyEvent) {
	if event.Name == fyne.KeyEscape {
		e.finish(false)
		return
	}
	e.Entry.TypedKey(event)
}
