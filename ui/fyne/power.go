package fyneui

import (
	"fyne.io/fyne/v2/widget"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func renderPower(indicator *widget.Button, track string, state p.PowerState, connected bool) {
	known := state != ""
	if state == "" {
		state = "UNKNOWN"
	}
	indicator.Importance = widget.MediumImportance
	if state == p.On {
		indicator.Importance = widget.HighImportance
	}
	indicator.SetText(track + ": " + string(state))
	if connected && known {
		indicator.Enable()
	} else {
		indicator.Disable()
	}
}

func (v *View) renderPowerControls(s th.State) {
	renderPower(v.mainPower, "MAIN", s.MainPower, s.Connected)
	renderPower(v.progPower, "PROG", s.ProgPower, s.Connected)
	for _, b := range []*widget.Button{v.allOn, v.allOff} {
		if s.Connected {
			b.Enable()
		} else {
			b.Disable()
		}
	}
}
