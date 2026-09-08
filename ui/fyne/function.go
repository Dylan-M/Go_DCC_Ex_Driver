package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

// FunctionButton supports a held momentary action, one-edge toggle action,
// keyboard activation, and touch cancellation. Tapped after MouseUp must not
// generate a second pair of commands.
type FunctionButton struct {
	widget.Button
	Press, Release, ChangeMode func()
	held, suppressTap          bool
}

func newFunctionButton(label string, press, release, mode func()) *FunctionButton {
	b := &FunctionButton{Press: press, Release: release, ChangeMode: mode}
	b.Text = label
	b.ExtendBaseWidget(b)
	return b
}
func (b *FunctionButton) down() {
	if b.Disabled() || b.held {
		return
	}
	b.held = true
	b.suppressTap = true
	if b.Press != nil {
		b.Press()
	}
}
func (b *FunctionButton) up() {
	if !b.held {
		return
	}
	b.held = false
	if b.Release != nil {
		b.Release()
	}
}
func (b *FunctionButton) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.down()
	}
}
func (b *FunctionButton) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.up()
	}
}
func (b *FunctionButton) MouseOut()                      { b.Button.MouseOut(); b.up() }
func (b *FunctionButton) TouchDown(*mobile.TouchEvent)   { b.down() }
func (b *FunctionButton) TouchUp(*mobile.TouchEvent)     { b.up() }
func (b *FunctionButton) TouchCancel(*mobile.TouchEvent) { b.up() }
func (b *FunctionButton) Tapped(*fyne.PointEvent) {
	if b.suppressTap {
		b.suppressTap = false
		return
	}
	b.down()
	b.up()
	b.suppressTap = false
}
func (b *FunctionButton) TappedSecondary(*fyne.PointEvent) {
	if b.ChangeMode != nil {
		b.ChangeMode()
	}
}
func (b *FunctionButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeySpace || e.Name == fyne.KeyReturn {
		b.suppressTap = false
		b.Tapped(nil)
	}
}
