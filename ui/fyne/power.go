package fyneui

import (
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
)

func renderPower(indicator *canvas.Text, track string, state p.PowerState) {
	if state == "" {
		state = "UNKNOWN"
	}
	indicator.Text = track + ": " + string(state)
	indicator.Color = theme.Color(theme.ColorNameForeground)
	switch state {
	case p.On:
		indicator.Color = green
	case p.Overload:
		indicator.Color = red
	case "MIXED":
		indicator.Color = orange
	}
	indicator.Refresh()
}
