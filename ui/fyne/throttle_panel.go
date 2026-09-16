package fyneui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"image/color"
	"strconv"
	"time"
)

type throttlePanel struct {
	owner      *View
	address    int
	rendering  bool
	last       th.State
	tab        *container.TabItem
	cab        *widget.Entry
	direction  *DirectionSwitch
	speed      *widget.Slider
	speedLabel *widget.Label
	functions  [29]*FunctionButton
	lamps      [29]*canvas.Rectangle
}

func (t *throttlePanel) post(fn func(*th.Controller) error) {
	cab := t.address
	t.owner.post(func(c *th.Controller) error { return c.WithCab(cab, fn) })
}
func (t *throttlePanel) showError(err error) { t.owner.showError(err) }
func (t *throttlePanel) integer(e *widget.Entry, label string) (int, bool) {
	return t.owner.integer(e, label)
}
func (t *throttlePanel) build() fyne.CanvasObject {
	t.cab = entry(strconv.Itoa(t.address))
	t.cab.OnSubmitted = func(text string) {
		if n, ok := t.integer(t.cab, "Address"); ok {
			t.owner.post(func(c *th.Controller) error { return c.ReplaceCab(t.address, n) })
		}
	}
	selectCab := widget.NewButton("Select", func() { t.cab.OnSubmitted(t.cab.Text) })
	t.direction = newDirectionSwitch(func(selected string) {
		if t.rendering {
			return
		}
		dir := 0
		if selected == "Fwd" {
			dir = 1
		}
		t.post(func(c *th.Controller) error { return c.SetDirection(dir, time.Now()) })
	})
	t.direction.Disable()
	stop := widget.NewButton("STOP", func() { t.post(func(c *th.Controller) error { return c.Stop(time.Now()) }) })
	estop := widget.NewButton("E-STOP ALL", func() { t.post(func(c *th.Controller) error { return c.Emergency() }) })
	t.speedLabel = widget.NewLabel("Speed: 0")
	t.speed = widget.NewSlider(0, 126)
	t.speed.Step = 1
	t.speed.OnChanged = func(value float64) {
		if !t.rendering {
			t.speedLabel.SetText(fmt.Sprintf("Speed: %d", int(value)))
			t.post(func(c *th.Controller) error { return c.MoveSpeed(int(value)) })
		}
	}
	controls := container.NewGridWithColumns(4, container.NewBorder(nil, nil, nil, selectCab, t.cab), t.direction, colored(stop, orange), colored(estop, red))
	var funcs []fyne.CanvasObject
	for n := 0; n < 29; n++ {
		t.functions[n] = newFunctionButton(fmt.Sprintf("F%d", n), func() { t.post(func(c *th.Controller) error { return c.Function(n, true) }) }, func() { t.post(func(c *th.Controller) error { return c.Function(n, false) }) }, func() { t.showError(t.owner.session.SetToggle(n, !t.last.Toggle[n])) })
		lamp := canvas.NewRectangle(color.Transparent)
		lamp.StrokeWidth = 3
		t.lamps[n] = lamp
		funcs = append(funcs, container.NewStack(lamp, container.NewPadded(t.functions[n])))
	}
	modes := widget.NewButton("Function modes…", func() {
		var checks []fyne.CanvasObject
		for n := 0; n < 29; n++ {
			check := widget.NewCheck(fmt.Sprintf("F%d toggle", n), nil)
			check.SetChecked(t.last.Toggle[n])
			check.OnChanged = func(on bool) { t.showError(t.owner.session.SetToggle(n, on)) }
			checks = append(checks, check)
		}
		dialog.ShowCustom("Function modes", "Done", container.NewGridWithColumns(4, checks...), t.owner.Window)
	})
	body := container.NewVBox(widget.NewLabel("Locomotive address"), controls, t.speedLabel, t.speed, widget.NewSeparator(),
		widget.NewLabel("Functions — hold for momentary; right-click to change mode"), container.NewGridWithColumns(10, funcs...),
		container.NewHBox(widget.NewButton("All Functions Off", func() { t.post(func(c *th.Controller) error { return c.AllFunctionsOff() }) }), modes))
	return container.NewVScroll(body)
}

func (t *throttlePanel) render(global th.State, cab th.CabState) {
	t.rendering = true
	defer func() { t.rendering = false }()
	t.speed.SetValue(float64(cab.Speed))
	t.speedLabel.SetText(fmt.Sprintf("Speed: %d", cab.Speed))
	if cab.Cab != t.last.Cab {
		t.cab.SetText(strconv.Itoa(cab.Cab))
	}
	selected := "Rev"
	if cab.Direction == 1 {
		selected = "Fwd"
	}
	t.direction.SetSelected(selected)
	if global.Connected {
		t.direction.Enable()
	} else {
		t.direction.Disable()
	}
	for n, b := range t.functions {
		label := fmt.Sprintf("F%d", n)
		if global.Toggle[n] {
			label += " ↕"
		}
		b.SetText(label)
		t.lamps[n].StrokeColor = color.Transparent
		if cab.Functions[n] {
			t.lamps[n].StrokeColor = green
		}
		t.lamps[n].Refresh()
	}
	t.last = global
	t.last.Cab, t.last.Speed, t.last.Direction, t.last.Functions = cab.Cab, cab.Speed, cab.Direction, cab.Functions
}
