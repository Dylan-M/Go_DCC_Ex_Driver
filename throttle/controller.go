// Package throttle implements GUI-independent state and safety rules.
// One event loop owns a Controller; its methods must not run concurrently.
package throttle

import (
	"context"
	"errors"
	"fmt"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/capabilities"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	"go.opentelemetry.io/otel/attribute"
	olog "go.opentelemetry.io/otel/log"
	"strings"
	"time"
)

type Sender interface {
	Send(string) error
	Close() error
}
type LogEntry struct{ Kind, Text string }
type State struct {
	Discovery                                   capabilities.Snapshot
	Throttles                                   []CabState
	Connected                                   bool
	ActiveConnection                            Connection
	Status                                      string
	Cab, Speed, Direction                       int
	Functions, Toggle                           [29]bool
	Power                                       string
	MainPower, ProgPower                        p.PowerState
	Overload                                    bool
	CurrentMA, MaxMA, TripMA                    int
	HasCurrent, HasLimits                       bool
	Poll                                        bool
	ProgramResult, ProgramAddress, ProgramValue string
	CV29                                        byte
	CV29Known                                   bool
	Logs                                        []LogEntry
}
type Controller struct {
	discovery                capabilities.Discovery
	telemetry                *telemetry.Manager
	operationContext         context.Context
	pendingContext           context.Context
	cabs                     map[int]cabRuntime
	order                    []int
	state                    State
	sender                   Sender
	pending                  *int
	lastSpeed, lastDirection int
	lastKnown                bool
	lastSent                 time.Time
	trackModes               [8]string
	trackPowers              [8]p.PowerState
	defaultToggle            [29]bool
}

func New(toggle [29]bool) *Controller {
	return &Controller{defaultToggle: toggle, order: []int{3}, cabs: map[int]cabRuntime{3: {state: CabState{Cab: 3, Direction: 1, Toggle: toggle}}}, state: State{Status: "Disconnected", Cab: 3, Direction: 1, Power: "power: unknown", Poll: true, ProgramResult: "result: --", Toggle: toggle}}
}
func (c *Controller) Snapshot() State {
	s := c.state
	s.Discovery = c.discovery.Snapshot()
	s.Logs = append([]LogEntry(nil), s.Logs...)
	s.Throttles = c.cabStates()
	return s
}
func (c *Controller) Log(kind, text string) {
	ctx := c.operationContext
	if ctx == nil {
		ctx = context.Background()
	}
	switch kind {
	case "tx", "rx": // Raw traffic is emitted once by the client, at debug only.
	case "err":
		c.telemetry.Log(ctx, olog.SeverityError, "Controller operation failed")
		c.telemetry.Log(ctx, olog.SeverityDebug, telemetry.Redact(text))
	default:
		c.telemetry.Log(ctx, olog.SeverityInfo, telemetry.Redact(text))
	}
	if len(text) > 8192 {
		text = text[:8192] + "…"
	}
	c.state.Logs = append(c.state.Logs, LogEntry{kind, text})
	if len(c.state.Logs) > 500 {
		copy(c.state.Logs, c.state.Logs[len(c.state.Logs)-500:])
		c.state.Logs = c.state.Logs[:500]
	}
}
func (c *Controller) Attach(s Sender, description string) error {
	c.Detach("Disconnected")
	c.sender = s
	c.state.Connected = true
	c.state.Status = "Connected to " + description
	c.pending = nil
	c.lastKnown = false
	c.Log("info", c.state.Status)
	if err := c.send(p.EncodeTrackQuery(), false); err != nil {
		return err
	}
	if err := c.send(p.EncodeStatus(), false); err != nil {
		return err
	}
	for _, cab := range c.cabStates() {
		cmd, _ := p.EncodeLocoRequest(cab.Cab)
		if err := c.send(cmd, false); err != nil {
			return err
		}
	}
	return nil
}
func (c *Controller) Detach(reason string) {
	c.discovery.Reset()
	if c.sender != nil {
		c.sender.Close()
	}
	c.sender = nil
	c.state.Connected = false
	c.state.ActiveConnection = Connection{}
	c.state.Status = reason
	c.pending = nil
	c.lastKnown = false
	c.state.Power = "power: unknown"
	for cab, state := range c.cabs {
		state.pending = nil
		state.lastKnown = false
		state.held = [29]bool{}
		c.cabs[cab] = state
	}
	c.state.MainPower, c.state.ProgPower = "", ""
	c.trackModes = [8]string{}
	c.trackPowers = [8]p.PowerState{}
	c.state.Overload = false
	c.state.HasCurrent = false
	c.state.HasLimits = false
	c.state.CurrentMA = 0
	c.state.MaxMA = 0
	c.state.TripMA = 0
}
func (c *Controller) Close() error {
	// This client does not own the shared layout. Closing it must not change
	// track power or locomotive state for other operators.
	c.Detach("Disconnected")
	return nil
}
func (c *Controller) send(cmd string, quiet bool) (resultErr error) {
	// The parent action owns its target. A command can address another loco or
	// the whole station, so the selected tab is not a reliable command attribute.
	ctx, finish := c.telemetry.Start(c.operationContext, "command."+telemetry.CommandKind(cmd))
	defer func() { finish(resultErr) }()
	if c.sender == nil {
		err := errors.New("not connected")
		if !quiet {
			c.Log("err", err.Error())
		}
		return err
	}
	var err error
	if contextual, ok := c.sender.(interface {
		SendContext(context.Context, string) error
	}); ok {
		err = contextual.SendContext(ctx, cmd)
	} else {
		err = c.sender.Send(cmd)
	}
	if err != nil {
		c.Detach("Send failed: " + err.Error())
		c.Log("err", c.state.Status)
		return err
	}
	if !quiet {
		c.Log("tx", ">> "+cmd)
	}
	return nil
}
func (c *Controller) command(cmd string, err error) error {
	if err != nil {
		c.Log("err", err.Error())
		return err
	}
	return c.send(cmd, false)
}
func (c *Controller) SelectCab(cab int) error {
	return c.selectCab(cab, false)
}

func (c *Controller) selectCab(cab int, preservePending bool) error {
	cmd, err := p.EncodeLocoRequest(cab)
	if err != nil {
		return err
	}
	if cab == c.state.Cab {
		return nil
	}
	if !preservePending {
		c.pending = nil
	}
	c.storeCab()
	if _, ok := c.cabs[cab]; !ok {
		c.cabs[cab] = cabRuntime{state: CabState{Cab: cab, Direction: 1, Toggle: c.defaultToggle}}
		c.order = append(c.order, cab)
	}
	c.loadCab(cab)
	c.Log("info", fmt.Sprintf("loco %d selected", cab))
	if c.sender != nil {
		return c.send(cmd, false)
	}
	return nil
}
func (c *Controller) MoveSpeed(speed int) (resultErr error) {
	finish := c.operation("speed", attribute.Int("speed", speed))
	defer func() { finish(resultErr) }()
	if _, err := p.EncodeThrottle(c.state.Cab, speed, c.state.Direction); err != nil {
		return err
	}
	if speed < 0 {
		return errors.New("slider speed must be 0-126")
	}
	c.state.Speed = speed
	if c.sender != nil {
		v := speed
		c.pending = &v
		c.pendingContext = c.operationContext
	} else {
		c.pending = nil
	}
	return nil
}
func (c *Controller) transmit(speed, dir int, now time.Time) error {
	cmd, err := p.EncodeThrottle(c.state.Cab, speed, dir)
	if err != nil {
		return err
	}
	if err = c.send(cmd, false); err != nil {
		return err
	}
	c.lastSpeed = speed
	c.lastDirection = dir
	c.lastKnown = true
	c.lastSent = now
	return nil
}
func (c *Controller) tickCurrent(now time.Time) error {
	if c.pending == nil || c.sender == nil {
		return nil
	}
	speed := *c.pending
	if c.lastKnown && speed == c.lastSpeed && c.state.Direction == c.lastDirection {
		c.pending = nil
		return nil
	}
	if !c.lastSent.IsZero() && now.Sub(c.lastSent) < 80*time.Millisecond {
		return nil
	}
	c.pending = nil
	previousContext := c.operationContext
	c.operationContext = c.pendingContext
	defer func() { c.operationContext = previousContext; c.pendingContext = nil }()
	return c.transmit(speed, c.state.Direction, now)
}
func (c *Controller) Direction(now time.Time) error {
	return c.SetDirection(1-c.state.Direction, now)
}

// Explicit selection must not invert direction when a stale UI event is queued,
// or resend a command when the already-selected direction is tapped.
func (c *Controller) SetDirection(dir int, now time.Time) (resultErr error) {
	finish := c.operation("direction", attribute.Int("direction", dir))
	defer func() { finish(resultErr) }()
	if dir != 0 && dir != 1 {
		return errors.New("direction must be 0 (reverse) or 1 (forward)")
	}
	if dir == c.state.Direction {
		return nil
	}
	if err := c.transmit(c.state.Speed, dir, now); err != nil {
		return err
	}
	c.state.Direction = dir
	return nil
}
func (c *Controller) Stop(now time.Time) (resultErr error) {
	finish := c.operation("stop")
	defer func() { finish(resultErr) }()
	c.pending = nil
	if err := c.transmit(0, c.state.Direction, now); err != nil {
		return err
	}
	c.state.Speed = 0
	return nil
}
func (c *Controller) Emergency() (resultErr error) {
	finish := c.operation("emergency_stop")
	defer func() { finish(resultErr) }()
	c.pending = nil
	// Never replay another throttle's queued movement after a stop attempt,
	// even if the emergency command itself fails to reach the station.
	for cab, state := range c.cabs {
		state.pending = nil
		c.cabs[cab] = state
	}
	if err := c.send(p.EncodeEmergencyStop(), false); err != nil {
		return err
	}
	c.state.Speed = 0
	for cab, state := range c.cabs {
		state.state.Speed = 0
		state.pending = nil
		state.lastSpeed = 0
		c.cabs[cab] = state
	}
	c.lastSpeed = 0
	c.lastDirection = c.state.Direction
	c.lastKnown = true
	return nil
}
func (c *Controller) SetPoll(enabled bool) { c.state.Poll = enabled }
func (c *Controller) Poll() error {
	if c.state.Poll && c.sender != nil {
		return c.send(p.EncodeCurrentQuery(), true)
	}
	return nil
}
func (c *Controller) Power(on bool, track p.Track) (resultErr error) {
	finish := c.operation("power", attribute.Bool("on", on), attribute.String("track", string(track)))
	defer func() { finish(resultErr) }()
	return c.command(p.EncodePower(on, track))
}
func validFunction(n int) bool { return n >= 0 && n < 29 }
func (c *Controller) SetToggle(n int, on bool) (resultErr error) {
	finish := c.operation("function_mode", attribute.Int("function", n), attribute.Bool("toggle", on))
	defer func() { finish(resultErr) }()
	if !validFunction(n) {
		return errors.New("function must be F0-F28")
	}
	c.state.Toggle[n] = on
	return nil
}
func (c *Controller) Function(n int, pressed bool) (resultErr error) {
	finish := c.operation("function", attribute.Int("function", n), attribute.Bool("pressed", pressed))
	defer func() { finish(resultErr) }()
	if !validFunction(n) {
		return errors.New("function must be F0-F28")
	}
	// A momentary press still needs its matching release if the user changes
	// this function to toggle mode while holding the button.
	toggle := c.state.Toggle[n] && !(c.cabs[c.state.Cab].held[n] && !pressed)
	if toggle && !pressed {
		return nil
	}
	on := pressed
	if toggle {
		on = !c.state.Functions[n]
	}
	value := 0
	if on {
		value = 1
	}
	if err := c.command(p.EncodeFunction(c.state.Cab, n, value)); err != nil {
		return err
	}
	r := c.cabs[c.state.Cab]
	r.held[n] = pressed && !toggle
	c.cabs[c.state.Cab] = r
	if toggle {
		c.state.Functions[n] = on
	}
	return nil
}
func (c *Controller) AllFunctionsOff() (resultErr error) {
	finish := c.operation("functions_off")
	defer func() { finish(resultErr) }()
	for n, on := range c.state.Functions {
		if on {
			if err := c.command(p.EncodeFunction(c.state.Cab, n, 0)); err != nil {
				return err
			}
			c.state.Functions[n] = false
		}
	}
	return nil
}
func (c *Controller) ReadAddress() (resultErr error) {
	finish := c.operation("address_read")
	defer func() { finish(resultErr) }()
	if err := c.send(p.EncodeReadAddress(), false); err != nil {
		return err
	}
	c.state.ProgramResult = "reading address..."
	return nil
}
func (c *Controller) WriteAddress(address int) (resultErr error) {
	finish := c.operation("address_write", attribute.Int("address", address))
	defer func() { finish(resultErr) }()
	if err := c.command(p.EncodeWriteAddress(address)); err != nil {
		return err
	}
	c.state.ProgramResult = "writing address..."
	return nil
}
func (c *Controller) ReadCV(cv int) (resultErr error) {
	finish := c.operation("cv_read", attribute.Int("cv", cv))
	defer func() { finish(resultErr) }()
	if err := c.command(p.EncodeReadCV(cv)); err != nil {
		return err
	}
	c.state.ProgramResult = "reading " + CVDescription(cv) + "..."
	return nil
}
func (c *Controller) WriteCV(cv, value int) (resultErr error) {
	finish := c.operation("cv_write", attribute.Int("cv", cv), attribute.Int("value", value))
	defer func() { finish(resultErr) }()
	if err := c.command(p.EncodeWriteCV(cv, value)); err != nil {
		return err
	}
	c.state.ProgramResult = "writing " + CVDescription(cv) + "..."
	return nil
}
func (c *Controller) POM(cab, cv, value int) (resultErr error) {
	finish := c.operation("pom", attribute.Int("loco.address", cab), attribute.Int("cv", cv), attribute.Int("value", value))
	defer func() { finish(resultErr) }()
	return c.command(p.EncodeProgramOnMain(cab, cv, value))
}
func (c *Controller) EditCV29(low byte) { c.state.CV29 = c.state.CV29&0xc0 | low&0x3f }
func (c *Controller) WriteCV29() error  { return c.WriteCV(29, int(c.state.CV29)) }
func (c *Controller) Raw(text string) (resultErr error) {
	finish := c.operation("raw")
	defer func() { finish(resultErr) }()
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if !strings.HasPrefix(text, "<") {
		text = "<" + text + ">"
	}
	return c.send(text, false)
}
func (c *Controller) Receive(e p.Event) {
	if e == nil || !c.state.Connected {
		return
	}
	ctx, finish := c.telemetry.Start(c.operationContext, "controller.receive", attribute.String("command", telemetry.CommandKind(e.RawFrame())))
	defer finish(nil)
	_, current := e.(p.CurrentInfo)
	if !current || !c.state.Poll {
		c.Log("rx", "<< "+e.RawFrame())
	}
	switch v := e.(type) {
	case p.LocoState:
		if v.Cab != c.state.Cab {
			if _, ok := c.cabs[v.Cab]; ok {
				c.WithCab(v.Cab, func(c *Controller) error { c.receiveLoco(v); return nil })
			}
			return
		}
		c.receiveLoco(v)
	case p.TrackPower:
		c.telemetry.Event(ctx, "track.power", attribute.String("track", v.Track), attribute.String("state", string(v.State)))
		c.state.Power = fmt.Sprintf("power: %s %s", v.Track, v.State)
		c.receivePower(v)
	case p.TrackMode:
		if len(v.Track) == 1 && v.Track[0] >= 'A' && v.Track[0] <= 'H' {
			c.trackModes[v.Track[0]-'A'] = v.Mode
			c.state.MainPower, c.state.ProgPower = "", ""
			c.updateTrackPower()
		}
	case p.CurrentInfo:
		c.telemetry.Current(v.CurrentMA)
		c.state.CurrentMA = v.CurrentMA
		c.state.HasCurrent = true
		if v.HasLimits {
			c.state.MaxMA = v.MaxMA
			c.state.TripMA = v.TripMA
			c.state.HasLimits = true
		}
	case p.VersionInfo:
		c.state.Status = v.Text
	case p.CVResult:
		c.telemetry.Event(ctx, "programming.cv_result", attribute.String("operation", string(v.Operation)), attribute.Int("cv", v.CV), attribute.Bool("success", v.Success()))
		desc := CVDescription(v.CV)
		if !v.Success() {
			c.state.ProgramResult = desc + " " + string(v.Operation) + " FAILED"
			return
		}
		if v.Operation == p.Read {
			c.state.ProgramResult = fmt.Sprintf("%s = %d", desc, v.Value)
			c.state.ProgramValue = fmt.Sprint(v.Value)
		} else {
			c.state.ProgramResult = fmt.Sprintf("%s written: %d", desc, v.Value)
		}
		if v.CV == 29 {
			c.state.CV29 = byte(v.Value)
			c.state.CV29Known = true
		}
	case p.AddressResult:
		c.telemetry.Event(ctx, "programming.address_result", attribute.String("operation", string(v.Operation)), attribute.Bool("success", v.Success()))
		if !v.Success() {
			c.state.ProgramResult = "address " + string(v.Operation) + " FAILED"
			return
		}
		if v.Operation == p.Read {
			c.state.ProgramResult = fmt.Sprintf("loco address = %d", v.Address)
			c.state.ProgramAddress = fmt.Sprint(v.Address)
		} else {
			c.state.ProgramResult = fmt.Sprintf("address written: %d", v.Address)
		}
	}
}
