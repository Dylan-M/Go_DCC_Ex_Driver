package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"image/color"
)

var (
	powerBlue   = color.NRGBA{R: 21, G: 101, B: 192, A: 255}
	powerOrange = color.NRGBA{R: 255, G: 152, B: 0, A: 255}
	powerSlate  = color.NRGBA{R: 70, G: 82, B: 94, A: 255}
	powerTaupe  = color.NRGBA{R: 109, G: 94, B: 80, A: 255}
)

// Use dark text on vivid orange and white text on the darker button colors.
type powerTheme struct {
	fyne.Theme
	accent color.Color
}

func (t powerTheme) Color(n fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNamePrimary:
		return t.accent
	case theme.ColorNameForegroundOnPrimary:
		if t.accent == powerOrange {
			return color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		}
		return color.White
	default:
		return t.Theme.Color(n, variant)
	}
}

func powerAction(button *widget.Button, accent color.Color) *container.ThemeOverride {
	button.Importance = widget.HighImportance
	return container.NewThemeOverride(button, powerTheme{theme.DefaultTheme(), accent})
}

func renderPowerColor(wrapper *container.ThemeOverride, state p.PowerState, on, off color.Color) {
	accent := on
	if state == p.Off {
		accent = off
	}
	if wrapper.Theme.(powerTheme).accent != accent {
		wrapper.Theme = powerTheme{theme.DefaultTheme(), accent}
		wrapper.Refresh()
	}
}

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
	if state == p.On || state == p.Off {
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
	renderPowerColor(v.mainPowerTheme, s.MainPower, green, red)
	renderPowerColor(v.progPowerTheme, s.ProgPower, powerBlue, powerOrange)
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
