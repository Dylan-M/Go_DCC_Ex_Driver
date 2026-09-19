package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
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
			t.owner.post(func(c *th.Controller) error { return c.RenameCab(cab, name) })
		}
	}
	body := container.NewVBox(widget.NewForm(widget.NewFormItem("Loco name", t.name)), widget.NewLabel("Names save automatically. Maximum 80 characters."))
	t.setup = dialog.NewCustom("Locomotive setup", "Done", body, t.owner.Window)
	t.setup.SetOnClosed(func() { t.setup = nil })
	t.setup.Resize(fyne.NewSize(480, 180))
	t.setup.Show()
	t.owner.Window.Canvas().Focus(t.name)
}
