//go:build firmware

package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/client"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/transport"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

type notification struct {
	Port   int  `json:"port"`
	Closed bool `json:"closed"`
}
type firmware struct {
	port    int
	control io.WriteCloser
	events  <-chan notification
}

func startFirmware(t *testing.T) *firmware {
	t.Helper()
	// Explicit opt-in via build tag; missing dependencies must fail, never skip CI.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node.js 24 is required:", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	cmd := exec.CommandContext(ctx, node, "server.cjs")
	cmd.Dir = "emulator"
	cmd.Env = os.Environ()
	if file := os.Getenv("DCCEX_FIRMWARE"); file != "" {
		absolute, err := filepath.Abs(file)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		cmd.Env = append(cmd.Env, "DCCEX_FIRMWARE="+absolute)
	}
	logDir := os.Getenv("DCCEX_LOG_DIR")
	if logDir == "" {
		logDir = t.TempDir()
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		cancel()
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, t.Name()+".jsonl")
	log, err := os.Create(logPath)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cmd.Stderr = log
	out, err := cmd.StdoutPipe()
	if err != nil {
		log.Close()
		cancel()
		t.Fatal(err)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		log.Close()
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Close()
		cancel()
		t.Fatal(err)
	}
	events := make(chan notification, 32)
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		defer close(events)
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			var n notification
			if json.Unmarshal(scanner.Bytes(), &n) == nil {
				select {
				case events <- n:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	done := make(chan error, 1)
	go func() { <-scanDone; done <- cmd.Wait() }()
	t.Cleanup(func() {
		fmt.Fprintln(input, "quit")
		input.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("emulator exit: %v", err)
			}
		case <-time.After(5 * time.Second):
			cancel()
			<-done
			t.Error("emulator failed to shut down")
		}
		cancel()
		log.Close()
		if t.Failed() {
			data, _ := os.ReadFile(logPath)
			t.Logf("Firmware UART transcript (%s):\n%s", logPath, data)
		}
	})
	f := &firmware{control: input, events: events}
	select {
	case n, ok := <-events:
		if !ok || n.Port < 1 {
			t.Fatal("emulator did not report readiness")
		}
		f.port = n.Port
	case <-time.After(20 * time.Second):
		t.Fatal("emulator boot timed out")
	}
	return f
}
func (f *firmware) closed(t *testing.T) {
	t.Helper()
	select {
	case n, ok := <-f.events:
		if !ok || !n.Closed {
			t.Fatal("missing socket close notification")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("emulator did not drain/close connection")
	}
}

// Force the production decoder through every byte boundary, even if TCP itself
// coalesces the emulator's small UART output writes.
type byteReads struct{ net.Conn }

func (c byteReads) Read(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return c.Conn.Read(b)
}
func (f *firmware) connect(t *testing.T) *client.Client {
	t.Helper()
	conn, err := transport.TCP(context.Background(), "127.0.0.1", f.port)
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(byteReads{conn})
	t.Cleanup(func() { c.Close() })
	return c
}
func awaitEvent(t *testing.T, c *client.Client, match func(p.Event) bool) p.Event {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case r, ok := <-c.Events():
			if !ok {
				t.Fatal("connection closed before expected reply")
			}
			if r.Err != nil {
				t.Fatal(r.Err)
			}
			if r.Result.Err != nil {
				t.Fatal(r.Result.Err)
			}
			if match(r.Result.Event) {
				return r.Result.Event
			}
		case <-timer.C:
			t.Fatal("timed out waiting for firmware reply")
		}
	}
}
func send(t *testing.T, c *client.Client, cmd string) {
	t.Helper()
	if err := c.Send(cmd); err != nil {
		t.Fatal(err)
	}
}
func encoded(t *testing.T, cmd string, err error) string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}
func loco(t *testing.T, c *client.Client, cab, speed, dir int, mask uint32, emergency bool) {
	t.Helper()
	awaitEvent(t, c, func(e p.Event) bool {
		v, ok := e.(p.LocoState)
		return ok && v.Cab == cab && v.Speed == speed && v.Direction == dir && v.FunctionMask == mask && v.Emergency == emergency
	})
}
func TestFirmwareClientFragmented(t *testing.T) {
	f := startFirmware(t)
	c := f.connect(t)
	send(t, c, p.EncodeStatus())
	awaitEvent(t, c, func(e p.Event) bool {
		v, ok := e.(p.VersionInfo)
		return ok && strings.Contains(v.Text, "V-5.6.1 / MEGA")
	})
	cmd, err := p.EncodeLocoRequest(3)
	send(t, c, encoded(t, cmd, err))
	loco(t, c, 3, 0, 1, 0, false)
	cmd, err = p.EncodeThrottle(3, 126, 0)
	send(t, c, encoded(t, cmd, err))
	loco(t, c, 3, 126, 0, 0, false)
	cmd, err = p.EncodeFunction(3, 28, 1)
	send(t, c, encoded(t, cmd, err))
	loco(t, c, 3, 126, 0, 1<<28, false)
	send(t, c, p.EncodeEmergencyStop())
	cmd, err = p.EncodeLocoRequest(3)
	send(t, c, encoded(t, cmd, err))
	loco(t, c, 3, 0, 0, 1<<28, true)
	send(t, c, p.EncodeCurrentQuery())
	awaitEvent(t, c, func(e p.Event) bool {
		v, ok := e.(p.CurrentInfo)
		return ok && v.CurrentMA == 0 && v.HasLimits && v.TripMA > 0
	})
	c.Close()
	f.closed(t)
}
func awaitState(t *testing.T, s *throttle.Session, match func(throttle.State) bool) throttle.State {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	var last throttle.State
	for {
		select {
		case v, ok := <-s.Updates():
			if !ok {
				t.Fatal("session ended before expected state")
			}
			last = v
			for _, entry := range v.Logs {
				if entry.Kind == "err" {
					t.Fatalf("session error: %s", entry.Text)
				}
			}
			if match(v) {
				return v
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for state: %+v", last)
		}
	}
}
func hasRX(s throttle.State, fragment string) bool {
	for _, l := range s.Logs {
		if l.Kind == "rx" && strings.Contains(l.Text, fragment) {
			return true
		}
	}
	return false
}
func post(t *testing.T, s *throttle.Session, fn func(*throttle.Controller) error) {
	t.Helper()
	if err := s.Post(fn); err != nil {
		t.Fatal(err)
	}
}
func session(t *testing.T, f *firmware) *throttle.Session {
	t.Helper()
	s := throttle.NewSession(config.Default(), nil, nil)
	t.Cleanup(func() {
		s.Close()
		select {
		case <-s.Done():
		case <-time.After(5 * time.Second):
			t.Error("session shutdown timed out")
		}
	})
	if err := s.Connect(throttle.Connection{Host: "127.0.0.1", Port: f.port}); err != nil {
		t.Fatal(err)
	}
	awaitState(t, s, func(v throttle.State) bool { return v.Connected && hasRX(v, "<l 3 -1 128 0>") })
	return s
}
func TestFirmwareSessionControlsAndShutdown(t *testing.T) {
	f := startFirmware(t)
	s := session(t, f)
	post(t, s, func(c *throttle.Controller) error { return c.MoveSpeed(25) })
	awaitState(t, s, func(v throttle.State) bool { return v.Speed == 25 && hasRX(v, "154 0>") })
	post(t, s, func(c *throttle.Controller) error { return c.Function(0, true) })
	awaitState(t, s, func(v throttle.State) bool { return v.Functions[0] && hasRX(v, "154 1>") })
	post(t, s, func(c *throttle.Controller) error { return c.Function(0, false) })
	awaitState(t, s, func(v throttle.State) bool { return !v.Functions[0] && v.Speed == 25 })
	post(t, s, func(c *throttle.Controller) error { return c.Direction(time.Now()) })
	awaitState(t, s, func(v throttle.State) bool { return v.Direction == 0 && hasRX(v, "26 0>") })
	post(t, s, func(c *throttle.Controller) error { return c.Stop(time.Now()) })
	awaitState(t, s, func(v throttle.State) bool { return v.Speed == 0 && hasRX(v, "<l 3 0 0 0>") })
	post(t, s, func(c *throttle.Controller) error { return c.SelectCab(7) })
	awaitState(t, s, func(v throttle.State) bool { return v.Cab == 7 && hasRX(v, "<l 7 -1 128 0>") })
	post(t, s, func(c *throttle.Controller) error { return c.Power(true, p.All) })
	awaitState(t, s, func(v throttle.State) bool { return hasRX(v, "<p1>") })
	s.Close()
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("session close timed out")
	}
	f.closed(t)
	// Query the still-running firmware through a fresh production client. This
	// proves Close's <0> reached firmware, rather than just changing local UI state.
	observer := f.connect(t)
	send(t, observer, p.EncodeStatus())
	awaitEvent(t, observer, func(e p.Event) bool { v, ok := e.(p.TrackPower); return ok && v.Track == "ALL" && v.State == p.Off })
	observer.Close()
	f.closed(t)
}
func TestFirmwareSessionDisconnect(t *testing.T) {
	f := startFirmware(t)
	s := session(t, f)
	if _, err := fmt.Fprintln(f.control, "disconnect"); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case v, ok := <-s.Updates():
			if !ok {
				t.Fatal("session ended before reporting disconnect")
			}
			if !v.Connected && strings.HasPrefix(v.Status, "Disconnected:") {
				return
			}
		case <-timer.C:
			t.Fatal("session did not notice lost firmware connection")
		}
	}
}

func TestFirmwareIndependentPowerIndicators(t *testing.T) {
	f := startFirmware(t)
	s := session(t, f)
	awaitState(t, s, func(v throttle.State) bool { return v.MainPower == p.Off && v.ProgPower == p.Off })
	for _, step := range []struct {
		track      p.Track
		on         bool
		main, prog p.PowerState
	}{
		{p.All, true, p.On, p.On},
		{p.Main, false, p.Off, p.On},
		{p.Prog, false, p.Off, p.Off},
		{p.Main, true, p.On, p.Off},
		{p.Prog, true, p.On, p.On},
		{p.All, false, p.Off, p.Off},
	} {
		post(t, s, func(c *throttle.Controller) error { return c.Power(step.on, step.track) })
		awaitState(t, s, func(v throttle.State) bool { return v.MainPower == step.main && v.ProgPower == step.prog })
	}
}

func TestFirmwareMultipleThrottles(t *testing.T) {
	f := startFirmware(t)
	s := session(t, f)
	post(t, s, func(c *throttle.Controller) error { return c.AddCab(7) })
	awaitState(t, s, func(v throttle.State) bool { return len(v.Throttles) == 2 && hasRX(v, "<l 7 -1 128 0>") })
	post(t, s, func(c *throttle.Controller) error {
		return c.WithCab(3, func(c *throttle.Controller) error { return c.MoveSpeed(25) })
	})
	post(t, s, func(c *throttle.Controller) error {
		return c.WithCab(7, func(c *throttle.Controller) error { return c.MoveSpeed(40) })
	})
	awaitState(t, s, func(v throttle.State) bool { return hasRX(v, "<l 3 0 154 0>") && hasRX(v, "<l 7 0 169 0>") })
	post(t, s, func(c *throttle.Controller) error { return c.FocusCab(3) })
	post(t, s, func(c *throttle.Controller) error {
		return c.WithCab(7, func(c *throttle.Controller) error { return c.SetDirection(0, time.Now()) })
	})
	awaitState(t, s, func(v throttle.State) bool {
		return v.Cab == 3 && v.Speed == 25 && v.Direction == 1 && v.Throttles[1].Direction == 0 && hasRX(v, "<l 7 0 41 0>")
	})
	post(t, s, func(c *throttle.Controller) error { return c.Emergency() })
	awaitState(t, s, func(v throttle.State) bool { return v.Throttles[0].Speed == 0 && v.Throttles[1].Speed == 0 })
}
