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
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"image/color"
	"strconv"
	"strings"
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
	*throttlePanel
	tabs      *container.AppTabs
	runTabs   *container.DocTabs
	panels    map[int]*throttlePanel
	pomTarget *widget.Label

	Window                                   fyne.Window
	session                                  *th.Session
	rendering                                bool
	last                                     th.State
	status, result, cv29Label                *widget.Label
	mainPower, progPower, allOn, allOff      *widget.Button
	current                                  *canvas.Text
	currentBar                               *widget.ProgressBar
	connect                                  *widget.Button
	host, port, baud, progAddress, progValue *widget.Entry
	devices                                  *widget.SelectEntry
	mode                                     *widget.Select
	bits                                     [6]*widget.Check
	console                                  *widget.List
	logs                                     []th.LogEntry
	stationStore                             stations.Repository
	savedStations                            *widget.Select
	saveStation, deleteStation               *widget.Button
	profiles                                 []stations.Profile
}

func entry(text string) *widget.Entry { e := widget.NewEntry(); e.SetText(text); return e }

type Options struct {
	Host     string
	Port     int
	Stations stations.Repository
}

func New(window fyne.Window, s *th.Session, options ...Options) *View {
	v := &View{Window: window, session: s, panels: make(map[int]*throttlePanel)}
	o := Options{Host: stations.DefaultHost, Port: stations.DefaultPort}
	if len(options) > 0 {
		o = options[0]
		if o.Host == "" {
			o.Host = stations.DefaultHost
		}
		if o.Port == 0 {
			o.Port = stations.DefaultPort
		}
	}
	v.stationStore = o.Stations
	v.status = widget.NewLabel("Disconnected")
	v.mainPower = widget.NewButton("Main (Unknown)", func() { v.post(func(c *th.Controller) error { return c.TogglePower(p.Main) }) })
	v.progPower = widget.NewButton("Prog (Unknown)", func() { v.post(func(c *th.Controller) error { return c.TogglePower(p.Prog) }) })
	v.allOn = widget.NewButton("All On", func() { v.post(func(c *th.Controller) error { return c.Power(true, p.All) }) })
	v.allOff = widget.NewButton("All Off", func() { v.post(func(c *th.Controller) error { return c.Power(false, p.All) }) })
	v.renderPowerControls(th.State{})
	v.result = widget.NewLabel("result: --")
	v.result.Wrapping = fyne.TextWrapWord
	v.current = canvas.NewText("current: --", theme.Color(theme.ColorNameForeground))
	v.current.TextSize = 14
	v.currentBar = widget.NewProgressBar()
	v.currentBar.Min = 0
	v.currentBar.Max = 1
	v.host = entry(o.Host)
	v.port = entry(strconv.Itoa(o.Port))
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
		container.NewHBox(refresh, v.status), v.stationControls())
	poll := widget.NewCheck("Poll current", func(on bool) {
		if !v.rendering {
			v.post(func(c *th.Controller) error { c.SetPoll(on); return nil })
		}
	})
	poll.SetChecked(true)
	power := container.NewVBox(container.NewGridWithColumns(4, v.allOn, v.allOff, v.mainPower, v.progPower), container.NewBorder(nil, nil, v.current, poll, v.currentBar))
	connection.Add(widget.NewSeparator())
	connection.Add(power)
	v.runTabs = container.NewDocTabs()
	v.syncThrottles(th.State{Cab: 3, Throttles: []th.CabState{{Cab: 3, Direction: 1}}})
	v.runTabs.OnSelected = func(tab *container.TabItem) {
		if v.rendering {
			return
		}
		for cab, panel := range v.panels {
			if panel.tab == tab {
				v.throttlePanel = panel
				v.post(func(c *th.Controller) error { return c.FocusCab(cab) })
				return
			}
		}
	}
	v.runTabs.CloseIntercept = func(tab *container.TabItem) {
		for cab, panel := range v.panels {
			if panel.tab == tab {
				for _, b := range panel.functions {
					b.up()
				}
				v.post(func(c *th.Controller) error { return c.RemoveCab(cab) })
				return
			}
		}
	}
	add := widget.NewButtonWithIcon("Throttle", theme.ContentAddIcon(), v.addThrottleDialog)
	v.runTabs.OnUnselected = func(tab *container.TabItem) {
		for _, panel := range v.panels {
			if panel.tab == tab {
				for _, b := range panel.functions {
					b.up()
				}
			}
		}
	}
	run := container.NewBorder(container.NewHBox(add), nil, nil, nil, v.runTabs)
	v.tabs = container.NewAppTabs(
		container.NewTabItem("Connection", container.NewVScroll(connection)),
		container.NewTabItem("Run", run),
		container.NewTabItem("Programming", v.programTab()))
	console := v.consoleView()
	split := container.NewVSplit(v.tabs, console)
	split.Offset = 0.72
	window.SetContent(split)
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
	v.pomTarget = widget.NewLabel("Program on Main — locomotive 3; no acknowledgement")
	pom := container.NewVBox(v.pomTarget,
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
	v.renderPowerControls(s)
	v.syncThrottles(s)
	v.pomTarget.SetText(fmt.Sprintf("Program on Main — locomotive %d; no acknowledgement", s.Cab))
	if s.Connected {
		v.connect.SetText("Disconnect")
	} else {
		v.connect.SetText("Connect")
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
