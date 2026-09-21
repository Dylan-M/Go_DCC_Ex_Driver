package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The outer renderer owns width-dependent header height. A VBox/Border based
// on the previous frame's MinSize would lag one layout behind on rotation.
type wrappedTabsRenderer struct {
	tabs      *locoTabs
	separator *widget.Separator
}

func (r *wrappedTabsRenderer) Layout(size fyne.Size) {
	t := r.tabs
	height := flowTabHeaders(t.bar.Objects, max(0, size.Width))
	t.bar.Layout.(*wrappedBarLayout).height = height
	t.bar.Resize(fyne.NewSize(size.Width, height))
	// Many restored tabs remain reachable without consuming the throttle area.
	headerHeight := min(height, max(0, size.Height*0.4))
	t.scroll.Move(fyne.NewPos(0, 0))
	t.scroll.Resize(fyne.NewSize(size.Width, headerHeight))
	t.scroll.Refresh()
	gap := theme.Padding()
	r.separator.Move(fyne.NewPos(0, headerHeight))
	r.separator.Resize(fyne.NewSize(size.Width, gap))
	t.content.Move(fyne.NewPos(0, headerHeight+gap))
	t.content.Resize(fyne.NewSize(size.Width, max(0, size.Height-headerHeight-gap)))
}

func (r *wrappedTabsRenderer) MinSize() fyne.Size {
	content := r.tabs.content.MinSize()
	return fyne.NewSize(max(224, content.Width), content.Height+48+theme.Padding())
}

func (r *wrappedTabsRenderer) Refresh() {
	r.Layout(r.tabs.Size())
	canvas.Refresh(r.tabs)
}

func (r *wrappedTabsRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.tabs.scroll, r.separator, r.tabs.content}
}

func (*wrappedTabsRenderer) Destroy() {}

// The outer renderer positions children; the scroll needs their full height,
// not the largest single child's MinSize returned by a layout-free container.
type wrappedBarLayout struct{ height float32 }

func (*wrappedBarLayout) Layout([]fyne.CanvasObject, fyne.Size) {}
func (l *wrappedBarLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, l.height)
}

// Tabs use their natural widths and are not subject to the two-button limit.
func flowTabHeaders(objects []fyne.CanvasObject, width float32) float32 {
	var x, y, rowHeight float32
	gap := theme.Padding()
	for _, child := range objects {
		if !child.Visible() {
			continue
		}
		size := child.MinSize()
		size.Width = min(size.Width, width)
		if x > 0 && x+size.Width > width {
			x = 0
			y += rowHeight + gap
			rowHeight = 0
		}
		child.Move(fyne.NewPos(x, y))
		child.Resize(size)
		rowHeight = max(rowHeight, size.Height)
		x += size.Width + gap
	}
	return y + rowHeight
}

func (t *locoTabs) revealHeader(header fyne.CanvasObject) {
	top, bottom := header.Position().Y, header.Position().Y+header.Size().Height
	offset := t.scroll.Offset.Y
	if top < offset {
		t.scroll.ScrollToOffset(fyne.NewPos(0, top))
	} else if bottom > offset+t.scroll.Size().Height {
		t.scroll.ScrollToOffset(fyne.NewPos(0, bottom-t.scroll.Size().Height))
	}
}
