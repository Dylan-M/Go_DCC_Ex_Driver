package fyneui

import (
	"fyne.io/fyne/v2/widget"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func renderPower(indicator *widget.Button, track string, state p.PowerState, connected bool) {
	known := state != ""
	label := "Unknown"
	switch state {
	case p.On:
		label = "On"
	case p.Off:
		label = "Off"
	case p.Overload:
		label = "Overload"
	case "MIXED":
		label = "Mixed"
	}
	indicator.Importance = widget.MediumImportance
	if state == p.On {
		indicator.Importance = widget.HighImportance
	}
	indicator.SetText(track + " (" + label + ")")
	if connected && known {
		indicator.Enable()
	} else {
		indicator.Disable()
	}
}

func (v *View) renderPowerControls(s th.State) {
	renderPower(v.mainPower, "Main", s.MainPower, s.Connected)
	renderPower(v.progPower, "Prog", s.ProgPower, s.Connected)
	for _, b := range []*widget.Button{v.allOn, v.allOff} {
		if s.Connected {
			b.Enable()
		} else {
			b.Disable()
		}
	}
}
