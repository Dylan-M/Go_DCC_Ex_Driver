package fyneui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
)

func (t *throttlePanel) showSetup() {
	if t.setup != nil {
		t.owner.Window.Canvas().Focus(t.name)
		return
	}
	t.name = entry(t.preferences.Name)
	t.name.SetPlaceHolder("Blank uses Loco <address>")
	t.name.Validator = config.ValidateDisplayName
	cab := t.address
	t.name.OnChanged = func(name string) {
		if t.name.Validate() == nil {
			t.showError(t.owner.session.RenameCab(cab, name))
		}
	}
	fields := widget.NewForm()
	for n := range t.labels {
		label := entry(t.preferences.Labels[n])
		label.SetPlaceHolder(fmt.Sprintf("F%d", n))
		label.Validator = config.ValidateDisplayName
		label.OnChanged = func(text string) {
			if label.Validate() == nil {
				t.showError(t.owner.session.SetFunctionLabel(cab, n, text))
			}
		}
		t.labels[n] = label
		mode := widget.NewCheck("Toggle", nil)
		mode.SetChecked(t.preferences.Toggle[n])
		mode.OnChanged = func(on bool) {
			if !t.rendering {
				t.showError(t.owner.session.SetFunctionToggle(cab, n, on))
			}
		}
		t.modes[n] = mode
		show := widget.NewCheck("Show", nil)
		show.SetChecked(!t.preferences.Hidden[n])
		show.OnChanged = func(on bool) {
			if !t.rendering {
				t.showError(t.owner.session.SetFunctionHidden(cab, n, !on))
			}
		}
		t.shown[n] = show
		var row fyne.CanvasObject = container.NewBorder(nil, nil, show, mode, label)
		if t.owner.mobile {
			row = container.NewVBox(label, container.NewGridWithColumns(2, show, mode))
		}
		fields.Append(fmt.Sprintf("F%d", n), row)
	}
	help := widget.NewLabel("Names and labels save automatically. Maximum 80 characters.")
	if t.owner.mobile {
		help.Wrapping = fyne.TextWrapWord
	}
	body := container.NewBorder(container.NewVBox(widget.NewForm(widget.NewFormItem("Loco name", t.name)), help), nil, nil, nil, container.NewVScroll(fields))
	t.setup = dialog.NewCustom("Locomotive setup", "Done", body, t.owner.Window)
	t.setup.SetOnClosed(func() { t.setup = nil })
	t.setup.Resize(fyne.NewSize(560, 480))
	if t.owner.mobile {
		size := t.owner.Window.Canvas().Size()
		t.setup.Resize(fyne.NewSize(min(560, size.Width-32), min(480, size.Height-64)))
	}
	t.setup.Show()
	t.owner.Window.Canvas().Focus(t.name)
}
