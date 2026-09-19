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
				t.owner.post(func(c *th.Controller) error { return c.SetFunctionLabel(cab, n, text) })
			}
		}
		t.labels[n] = label
		fields.Append(fmt.Sprintf("F%d label", n), label)
	}
	body := container.NewBorder(container.NewVBox(widget.NewForm(widget.NewFormItem("Loco name", t.name)), widget.NewLabel("Names and labels save automatically. Maximum 80 characters.")), nil, nil, nil, container.NewVScroll(fields))
	t.setup = dialog.NewCustom("Locomotive setup", "Done", body, t.owner.Window)
	t.setup.SetOnClosed(func() { t.setup = nil })
	t.setup.Resize(fyne.NewSize(560, 480))
	t.setup.Show()
	t.owner.Window.Canvas().Focus(t.name)
}
