// Package fyneui renders the throttle. It receives immutable state snapshots and
// submits intents; protocol and connection logic live outside this package.
package fyneui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/transport"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"image/color"
	"strconv"
	"strings"
	"time"
)

var green = color.NRGBA{R: 46, G: 125, B: 50, A: 255}
var orange = color.NRGBA{R: 239, G: 108, B: 0, A: 255}
var red = color.NRGBA{R: 198, G: 40, B: 40, A: 255}

type accentTheme struct {
	fyne.Theme
	accent color.Color
}

func (t accentTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNamePrimary {
		return t.accent
	}
	return t.Theme.Color(n, v)
}
func colored(b *widget.Button, c color.Color) fyne.CanvasObject {
	b.Importance = widget.HighImportance
	return container.NewThemeOverride(b, accentTheme{theme.DefaultTheme(), c})
}

type View struct {
	Window                                        fyne.Window
	session                                       *th.Session
	rendering                                     bool
	last                                          th.State
	status, power, speedLabel, result, cv29Label  *widget.Label
	current                                       *canvas.Text
	currentBar                                    *widget.ProgressBar
	speed                                         *widget.Slider
	direction, connect                            *widget.Button
	directionTheme                                *container.ThemeOverride
	cab, host, port, baud, progAddress, progValue *widget.Entry
	devices                                       *widget.SelectEntry
	mode                                          *widget.Select
	functions                                     [29]*FunctionButton
	lamps                                         [29]*canvas.Rectangle
	bits                                          [6]*widget.Check
	console                                       *widget.List
	logs                                          []th.LogEntry
}

func entry(text string) *widget.Entry { e := widget.NewEntry(); e.SetText(text); return e }
func New(window fyne.Window, s *th.Session) *View {
	v := &View{Window: window, session: s}
	v.status = widget.NewLabel("Disconnected")
	v.power = widget.NewLabel("power: unknown")
	v.result = widget.NewLabel("result: --")
	v.result.Wrapping = fyne.TextWrapWord
	v.current = canvas.NewText("current: --", theme.Color(theme.ColorNameForeground))
	v.current.TextSize = 14
	v.currentBar = widget.NewProgressBar()
	v.currentBar.Min = 0
	v.currentBar.Max = 1
	v.host = entry("192.168.4.1")
	v.port = entry("2560")
	v.baud = entry("115200")
	v.devices = widget.NewSelectEntry(nil)
	v.mode = widget.NewSelect([]string{"TCP", "Serial"}, nil)
	v.mode.SetSelected("TCP")
	v.connect = widget.NewButton("Connect", func() {
		o := th.Connection{Serial: v.mode.Selected == "Serial", Host: strings.TrimSpace(v.host.Text), Device: strings.TrimSpace(v.devices.Text)}
		var err error
		if o.Serial {
			o.Baud, err = strconv.Atoi(v.baud.Text)
		} else {
			o.Port, err = strconv.Atoi(v.port.Text)
		}
		if v.last.Connected {
			err = nil
		}
		if err != nil {
			v.showError(fmt.Errorf("invalid connection setting: %w", err))
			return
		}
		v.showError(s.Connect(o))
	})
	refresh := widget.NewButton("Refresh", func() {
		go func() {
			ports, err := transport.Ports()
			fyne.Do(func() {
				if err != nil {
					v.showError(err)
					return
				}
				v.devices.SetOptions(ports)
				if v.devices.Text == "" && len(ports) > 0 {
					v.devices.SetText(ports[0])
				}
			})
		}()
	})
	v.mode.OnChanged = func(mode string) {
		if mode == "Serial" {
			v.devices.Enable()
			v.baud.Enable()
			refresh.Enable()
			v.host.Disable()
			v.port.Disable()
		} else {
			v.devices.Disable()
			v.baud.Disable()
			refresh.Disable()
			v.host.Enable()
			v.port.Enable()
		}
	}
	v.mode.OnChanged("TCP")
	connection := container.NewVBox(container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewLabel("Connection"), nil, v.mode), v.connect),
		container.NewGridWithColumns(2, widget.NewForm(widget.NewFormItem("Host", v.host), widget.NewFormItem("TCP port", v.port)), widget.NewForm(widget.NewFormItem("Serial port", v.devices), widget.NewFormItem("Baud", v.baud))),
		container.NewHBox(refresh, v.status))
	var powers []fyne.CanvasObject
	for _, track := range []p.Track{p.All, p.Main, p.Prog} {
		for _, on := range []bool{true, false} {
			label := string(track) + " OFF"
			if on {
				label = string(track) + " ON"
			}
			powers = append(powers, widget.NewButton(label, func() { v.post(func(c *th.Controller) error { return c.Power(on, track) }) }))
		}
	}
	poll := widget.NewCheck("Poll current", func(on bool) {
		if !v.rendering {
			v.post(func(c *th.Controller) error { c.SetPoll(on); return nil })
		}
	})
	poll.SetChecked(true)
	power := container.NewVBox(container.NewGridWithColumns(6, powers...), v.power, container.NewBorder(nil, nil, v.current, poll, v.currentBar))
	tabs := container.NewAppTabs(container.NewTabItem("Run", v.runTab()), container.NewTabItem("Programming", v.programTab()))
	console := v.consoleView()
	split := container.NewVSplit(tabs, console)
	split.Offset = 0.72
	window.SetContent(container.NewBorder(container.NewVBox(connection, widget.NewSeparator(), power), nil, nil, nil, split))
	window.Resize(fyne.NewSize(1050, 840))
	return v
}
func (v *View) showError(err error) {
	if err != nil {
		dialog.ShowError(err, v.Window)
	}
}
func (v *View) post(fn func(*th.Controller) error) { v.showError(v.session.Post(fn)) }
func (v *View) integer(e *widget.Entry, label string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(e.Text))
	if err != nil {
		v.showError(fmt.Errorf("%s must be an integer", label))
		return 0, false
	}
	return n, true
}
func (v *View) runTab() fyne.CanvasObject {
	v.cab = entry("3")
	v.cab.OnSubmitted = func(text string) {
		if n, ok := v.integer(v.cab, "Address"); ok {
			v.post(func(c *th.Controller) error { return c.SelectCab(n) })
		}
	}
	selectCab := widget.NewButton("Select", func() { v.cab.OnSubmitted(v.cab.Text) })
	v.direction = widget.NewButton("FORWARD", func() { v.post(func(c *th.Controller) error { return c.Direction(time.Now()) }) })
	v.direction.Importance = widget.HighImportance
	v.directionTheme = container.NewThemeOverride(v.direction, accentTheme{theme.DefaultTheme(), green})
	stop := widget.NewButton("STOP", func() { v.post(func(c *th.Controller) error { return c.Stop(time.Now()) }) })
	estop := widget.NewButton("E-STOP ALL", func() { v.post(func(c *th.Controller) error { return c.Emergency() }) })
	v.speedLabel = widget.NewLabel("Speed: 0")
	v.speed = widget.NewSlider(0, 126)
	v.speed.Step = 1
	v.speed.OnChanged = func(value float64) {
		if !v.rendering {
			v.speedLabel.SetText(fmt.Sprintf("Speed: %d", int(value)))
			v.post(func(c *th.Controller) error { return c.MoveSpeed(int(value)) })
		}
	}
	controls := container.NewGridWithColumns(4, container.NewBorder(nil, nil, nil, selectCab, v.cab), v.directionTheme, colored(stop, orange), colored(estop, red))
	var funcs []fyne.CanvasObject
	for n := 0; n < 29; n++ {
		v.functions[n] = newFunctionButton(fmt.Sprintf("F%d", n), func() { v.post(func(c *th.Controller) error { return c.Function(n, true) }) }, func() { v.post(func(c *th.Controller) error { return c.Function(n, false) }) }, func() { v.showError(v.session.SetToggle(n, !v.last.Toggle[n])) })
		lamp := canvas.NewRectangle(color.Transparent)
		lamp.StrokeWidth = 3
		v.lamps[n] = lamp
		funcs = append(funcs, container.NewStack(lamp, container.NewPadded(v.functions[n])))
	}
	modes := widget.NewButton("Function modes…", func() {
		var checks []fyne.CanvasObject
		for n := 0; n < 29; n++ {
			check := widget.NewCheck(fmt.Sprintf("F%d toggle", n), nil)
			check.SetChecked(v.last.Toggle[n])
			check.OnChanged = func(on bool) { v.showError(v.session.SetToggle(n, on)) }
			checks = append(checks, check)
		}
		dialog.ShowCustom("Function modes", "Done", container.NewGridWithColumns(4, checks...), v.Window)
	})
	body := container.NewVBox(widget.NewLabel("Locomotive address"), controls, v.speedLabel, v.speed, widget.NewSeparator(),
		widget.NewLabel("Functions — hold for momentary; right-click to change mode"), container.NewGridWithColumns(10, funcs...),
		container.NewHBox(widget.NewButton("All Functions Off", func() { v.post(func(c *th.Controller) error { return c.AllFunctionsOff() }) }), modes))
	return container.NewVScroll(body)
}
func (v *View) programTab() fyne.CanvasObject {
	v.progAddress = entry("")
	v.progValue = entry("")
	cv := entry("")
	pomCV := entry("")
	pomValue := entry("")
	cvName, pomName := widget.NewLabel(""), widget.NewLabel("")
	cv.OnChanged = func(s string) { n, _ := strconv.Atoi(s); cvName.SetText(th.CVName(n)) }
	pomCV.OnChanged = func(s string) { n, _ := strconv.Atoi(s); pomName.SetText(th.CVName(n)) }
	readAddr := widget.NewButton("Read Address", func() { v.post(func(c *th.Controller) error { return c.ReadAddress() }) })
	writeAddr := widget.NewButton("Write Address", func() {
		if n, ok := v.integer(v.progAddress, "Address"); ok {
			v.post(func(c *th.Controller) error { return c.WriteAddress(n) })
		}
	})
	readCV := widget.NewButton("Read CV", func() {
		if n, ok := v.integer(cv, "CV"); ok {
			v.post(func(c *th.Controller) error { return c.ReadCV(n) })
		}
	})
	writeCV := widget.NewButton("Write CV", func() {
		n, ok := v.integer(cv, "CV")
		if !ok {
			return
		}
		value, ok := v.integer(v.progValue, "Value")
		if ok {
			v.post(func(c *th.Controller) error { return c.WriteCV(n, value) })
		}
	})
	service := container.NewVBox(widget.NewLabel("Programming Track — locomotive alone on PROG"),
		container.NewGridWithColumns(3, readAddr, v.progAddress, writeAddr),
		container.NewGridWithColumns(2, widget.NewForm(widget.NewFormItem("CV", cv)), widget.NewForm(widget.NewFormItem("Value", v.progValue))),
		cvName, container.NewHBox(readCV, writeCV), v.result)
	labels := []string{"Reverse direction", "28/128 speed steps", "Analog (DC) mode", "RailCom", "Custom speed table", "Long address (CV17/18)"}
	var bits []fyne.CanvasObject
	v.cv29Label = widget.NewLabel("CV29 = --")
	for n, label := range labels {
		v.bits[n] = widget.NewCheck(fmt.Sprintf("%s (b%d)", label, n), func(bool) {
			if v.rendering {
				return
			}
			var low byte
			for i, bit := range v.bits {
				if bit.Checked {
					low |= 1 << i
				}
			}
			v.cv29Label.SetText(fmt.Sprintf("CV29 = %d (not written)", v.last.CV29&0xc0|low))
			v.post(func(c *th.Controller) error { c.EditCV29(low); return nil })
		})
		bits = append(bits, v.bits[n])
	}
	editor := container.NewVBox(widget.NewLabel("CV29 Bit Editor"), container.NewGridWithColumns(3, bits...), container.NewHBox(v.cv29Label,
		widget.NewButton("Read CV29", func() { v.post(func(c *th.Controller) error { return c.ReadCV(29) }) }),
		widget.NewButton("Write CV29", func() { v.post(func(c *th.Controller) error { return c.WriteCV29() }) })),
		widget.NewLabel("Bit 5 selects the address type. Use Write Address to change an address."))
	pomWrite := widget.NewButton("Write on Main", func() {
		n, ok := v.integer(pomCV, "CV")
		if !ok {
			return
		}
		value, ok := v.integer(pomValue, "Value")
		if ok {
			v.post(func(c *th.Controller) error { return c.POM(n, value) })
		}
	})
	pom := container.NewVBox(widget.NewLabel("Program on Main — selected Run locomotive; no acknowledgement"),
		container.NewGridWithColumns(2, widget.NewForm(widget.NewFormItem("CV", pomCV)), widget.NewForm(widget.NewFormItem("Value", pomValue))), pomName, pomWrite)
	return container.NewVScroll(container.NewVBox(service, widget.NewSeparator(), editor, widget.NewSeparator(), pom))
}
func (v *View) consoleView() fyne.CanvasObject {
	v.console = widget.NewList(func() int { return len(v.logs) }, func() fyne.CanvasObject {
		text := canvas.NewText("", color.White)
		text.TextStyle.Monospace = true
		text.TextSize = 13
		return text
	}, func(i widget.ListItemID, o fyne.CanvasObject) {
		text := o.(*canvas.Text)
		line := v.logs[i]
		text.Text = line.Text
		switch line.Kind {
		case "err":
			text.Color = red
		case "tx":
			text.Color = color.NRGBA{136, 192, 208, 255}
		case "rx":
			text.Color = color.NRGBA{163, 190, 140, 255}
		default:
			text.Color = color.NRGBA{235, 203, 139, 255}
		}
		text.Refresh()
	})
	raw := entry("")
	send := func() { text := raw.Text; v.post(func(c *th.Controller) error { return c.Raw(text) }); raw.SetText("") }
	raw.OnSubmitted = func(string) { send() }
	return container.NewBorder(widget.NewLabel("Console"), container.NewBorder(nil, nil, widget.NewLabel("Raw:"), widget.NewButton("Send", send), raw), nil, nil, v.console)
}

// Render must run on Fyne's main goroutine.
func (v *View) Render(s th.State) {
	v.rendering = true
	defer func() { v.rendering = false }()
	v.status.SetText(s.Status)
	v.power.SetText(s.Power)
	v.speed.SetValue(float64(s.Speed))
	v.speedLabel.SetText(fmt.Sprintf("Speed: %d", s.Speed))
	if s.Connected {
		v.connect.SetText("Disconnect")
	} else {
		v.connect.SetText("Connect")
	}
	if s.Cab != v.last.Cab {
		v.cab.SetText(fmt.Sprint(s.Cab))
	}
	if s.Direction == 1 {
		v.direction.SetText("FORWARD")
		v.directionTheme.Theme = accentTheme{theme.DefaultTheme(), green}
	} else {
		v.direction.SetText("REVERSE")
		v.directionTheme.Theme = accentTheme{theme.DefaultTheme(), orange}
	}
	v.directionTheme.Refresh()
	for n, b := range v.functions {
		label := fmt.Sprintf("F%d", n)
		if s.Toggle[n] {
			label += " ↕"
		}
		b.SetText(label)
		v.lamps[n].StrokeColor = color.Transparent
		if s.Functions[n] {
			v.lamps[n].StrokeColor = green
		}
		v.lamps[n].Refresh()
	}
	limit := s.TripMA
	if limit == 0 {
		limit = s.MaxMA
	}
	v.currentBar.Max = 1
	v.currentBar.SetValue(0)
	v.current.Color = theme.Color(theme.ColorNameForeground)
	if s.Overload {
		v.current.Text = "OVERLOAD"
		v.current.Color = red
		v.currentBar.SetValue(1)
	} else if !s.HasCurrent {
		v.current.Text = "current: --"
	} else if limit > 0 {
		v.current.Text = fmt.Sprintf("%d mA / %d mA trip", s.CurrentMA, limit)
		v.currentBar.Max = float64(limit)
		v.currentBar.SetValue(float64(s.CurrentMA))
	} else {
		v.current.Text = fmt.Sprintf("%d mA", s.CurrentMA)
	}
	v.current.Refresh()
	v.result.SetText(s.ProgramResult)
	if s.ProgramAddress != v.last.ProgramAddress {
		v.progAddress.SetText(s.ProgramAddress)
	}
	if s.ProgramValue != v.last.ProgramValue {
		v.progValue.SetText(s.ProgramValue)
	}
	if s.CV29 != v.last.CV29 || s.CV29Known != v.last.CV29Known {
		for n, b := range v.bits {
			b.SetChecked(s.CV29&(1<<n) != 0)
		}
		v.cv29Label.SetText(fmt.Sprintf("CV29 = %d", s.CV29))
	}
	if len(s.Logs) != len(v.logs) || len(s.Logs) > 0 && s.Logs[len(s.Logs)-1] != v.logs[len(v.logs)-1] {
		v.logs = s.Logs
		v.console.Refresh()
		v.console.ScrollToBottom()
	}
	v.last = s
}
