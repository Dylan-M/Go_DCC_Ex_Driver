package fyneui

import (
	"image/color"
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestPowerButtonPalette(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("Power colors")
	s := th.NewSession(config.Default(), nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
	v := New(w, s, Options{PowerThrottle: true})
	connection := v.tabs.Items[0].Content.(*container.Scroll).Content.(*fyne.Container)
	power := connection.Objects[len(connection.Objects)-1].(*fyne.Container)
	buttons := power.Objects[0].(*fyne.Container)
	allOn := buttons.Objects[0].(*container.ThemeOverride)
	allOff := buttons.Objects[1].(*container.ThemeOverride)
	for _, state := range []struct {
		main, prog           p.PowerState
		mainColor, progColor color.Color
	}{
		{p.On, p.On, green, powerBlue},
		{p.Off, p.Off, red, powerOrange},
		{p.On, p.Off, green, powerOrange},
		{p.Off, p.On, red, powerBlue},
	} {
		v.renderPowerControls(th.State{Connected: true, MainPower: state.main, ProgPower: state.prog})
		wrappers := []*container.ThemeOverride{allOn, allOff, v.mainPowerTheme, v.progPowerTheme}
		want := []color.Color{powerSlate, powerTaupe, state.mainColor, state.progColor}
		for i, wrapper := range wrappers {
			for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
				if wrapper.Theme.Color(theme.ColorNamePrimary, variant) != want[i] {
					t.Fatalf("wrong color for button %d", i)
				}
				var foreground color.Color = color.White
				if want[i] == powerOrange {
					foreground = color.NRGBA{R: 24, G: 24, B: 24, A: 255}
				}
				if wrapper.Theme.Color(theme.ColorNameForegroundOnPrimary, variant) != foreground {
					t.Fatal("text must remain readable in both themes")
				}
			}
			for j := 0; j < i; j++ {
				if want[j] == want[i] {
					t.Fatal("buttons share a color")
				}
			}
		}
	}
	for _, state := range []p.PowerState{"", "MIXED", p.Overload} {
		v.renderPowerControls(th.State{Connected: true, MainPower: state, ProgPower: state})
		if v.mainPower.Importance != widget.MediumImportance || v.progPower.Importance != widget.MediumImportance {
			t.Fatal("unknown or fault state misleadingly uses on/off colors")
		}
	}
	v.renderPowerControls(th.State{})
	if !v.allOn.Disabled() || !v.allOff.Disabled() || !v.mainPower.Disabled() || !v.progPower.Disabled() {
		t.Fatal("disconnected power buttons enabled")
	}
	if path := os.Getenv("DCCEX_POWER_SCREENSHOT"); path != "" {
		v.renderPowerControls(th.State{Connected: true, MainPower: p.On, ProgPower: p.Off})
		w.Resize(fyne.NewSize(1050, 840))
		w.Show()
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
}
