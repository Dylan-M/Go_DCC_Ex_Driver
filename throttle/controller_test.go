package throttle_test

import (
	"errors"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"reflect"
	"testing"
	"time"
)

type sender struct {
	commands []string
	fail     bool
	closed   bool
}

func (s *sender) Send(cmd string) error {
	if s.fail {
		return errors.New("injected failure")
	}
	s.commands = append(s.commands, cmd)
	return nil
}
func (s *sender) Close() error { s.closed = true; return nil }
func setup(t *testing.T) (*th.Controller, *sender) {
	t.Helper()
	var modes [29]bool
	modes[3] = true
	ctrl := th.New(modes)
	s := &sender{}
	if err := ctrl.Attach(s, "test"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.commands, []string{"<s>", "<t 3>"}) {
		t.Fatal("handshake", s.commands)
	}
	s.commands = nil
	return ctrl, s
}
func receive(t *testing.T, c *th.Controller, frame string) {
	t.Helper()
	e, err := p.Parse(frame)
	if err != nil {
		t.Fatal(err)
	}
	c.Receive(e)
}
func TestOfflineSpeedNeverReplayed(t *testing.T) {
	c := th.New([29]bool{})
	c.MoveSpeed(90)
	s := &sender{}
	c.Attach(s, "test")
	c.Tick(time.Now())
	if len(s.commands) != 2 {
		t.Fatal("offline speed replayed", s.commands)
	}
	c.MoveSpeed(70)
	c.Detach("lost")
	s2 := &sender{}
	c.Attach(s2, "again")
	c.Tick(time.Now().Add(time.Second))
	if len(s2.commands) != 2 {
		t.Fatal("pending survived reconnect")
	}
}
func TestThrottleRateAndLatestIntent(t *testing.T) {
	c, s := setup(t)
	now := time.Unix(100, 0)
	c.MoveSpeed(10)
	c.Tick(now)
	c.MoveSpeed(20)
	c.Tick(now.Add(50 * time.Millisecond))
	c.MoveSpeed(30)
	c.Tick(now.Add(80 * time.Millisecond))
	if !reflect.DeepEqual(s.commands, []string{"<t 3 10 1>", "<t 3 30 1>"}) {
		t.Fatal(s.commands)
	}
	c.MoveSpeed(30)
	c.Tick(now.Add(time.Second))
	if len(s.commands) != 2 {
		t.Fatal("duplicate throttle")
	}
}
func TestObsoleteSpeedIsDiscarded(t *testing.T) {
	for _, action := range []string{"stop", "estop", "cab", "inbound", "disconnect"} {
		t.Run(action, func(t *testing.T) {
			c, s := setup(t)
			now := time.Now()
			c.MoveSpeed(90)
			switch action {
			case "stop":
				c.Stop(now)
			case "estop":
				c.Emergency()
			case "cab":
				c.SelectCab(4)
			case "inbound":
				receive(t, c, "<l 3 0 148 0>")
			case "disconnect":
				c.Detach("lost")
			}
			before := len(s.commands)
			c.Tick(now.Add(time.Second))
			if len(s.commands) != before {
				t.Fatal("stale speed sent", s.commands)
			}
		})
	}
}
func TestIncomingStateNeverEchoes(t *testing.T) {
	c, s := setup(t)
	receive(t, c, "<l 3 -1 148 9>")
	c.Tick(time.Now())
	state := c.Snapshot()
	if state.Speed != 19 || state.Direction != 1 || !state.Functions[0] || !state.Functions[3] || len(s.commands) != 0 {
		t.Fatalf("%+v %v", state, s.commands)
	}
	receive(t, c, "<l 4 0 255 0>")
	if c.Snapshot().Speed != 19 {
		t.Fatal("other cab changed selected loco")
	}
}
func TestSendFailureDoesNotClaimStopDirectionOrToggle(t *testing.T) {
	for _, action := range []string{"stop", "direction", "function", "estop"} {
		t.Run(action, func(t *testing.T) {
			c, s := setup(t)
			receive(t, c, "<l 3 0 148 8>")
			s.fail = true
			var err error
			switch action {
			case "stop":
				err = c.Stop(time.Now())
			case "direction":
				err = c.Direction(time.Now())
			case "function":
				err = c.Function(3, true)
			case "estop":
				err = c.Emergency()
			}
			if err == nil {
				t.Fatal("failure lost")
			}
			state := c.Snapshot()
			if state.Connected || state.Speed != 19 || state.Direction != 1 || !state.Functions[3] || !s.closed {
				t.Fatalf("false state %+v", state)
			}
		})
	}
}
func TestFunctionEdges(t *testing.T) {
	c, s := setup(t)
	c.Function(2, true)
	c.Function(2, false)
	c.Function(3, true)
	c.Function(3, false)
	c.Function(3, true)
	want := []string{"<F 3 2 1>", "<F 3 2 0>", "<F 3 3 1>", "<F 3 3 0>"}
	if !reflect.DeepEqual(s.commands, want) {
		t.Fatal(s.commands)
	}
	receive(t, c, "<l 3 0 128 5>")
	s.commands = nil
	c.AllFunctionsOff()
	if !reflect.DeepEqual(s.commands, []string{"<F 3 0 0>", "<F 3 2 0>"}) {
		t.Fatal(s.commands)
	}
}
func TestPowerCurrentAndQuietPolling(t *testing.T) {
	c, s := setup(t)
	initial := len(c.Snapshot().Logs)
	c.Poll()
	if !reflect.DeepEqual(s.commands, []string{"<c>"}) || len(c.Snapshot().Logs) != initial {
		t.Fatal("poll not quiet")
	}
	receive(t, c, "<p2 MAIN>")
	receive(t, c, "<c 10 400 600>")
	receive(t, c, "<c 0>")
	state := c.Snapshot()
	if !state.Overload || state.CurrentMA != 0 || state.TripMA != 600 {
		t.Fatalf("%+v", state)
	}
	receive(t, c, "<p0 MAIN>")
	if c.Snapshot().Overload || c.Snapshot().Power != "power: MAIN OFF" {
		t.Fatal("overload not cleared")
	}
	c.SetPoll(false)
	before := len(s.commands)
	c.Poll()
	if len(s.commands) != before {
		t.Fatal("poll disabled")
	}
	initial = len(c.Snapshot().Logs)
	receive(t, c, "<c 2>")
	if len(c.Snapshot().Logs) != initial+1 {
		t.Fatal("manual current hidden")
	}
}
func TestProgrammingAndCV29(t *testing.T) {
	c, s := setup(t)
	receive(t, c, "<v 29 192>")
	c.EditCV29(35)
	c.WriteCV29()
	if c.Snapshot().CV29 != 227 || s.commands[0] != "<W 29 227>" {
		t.Fatal("reserved bits lost")
	}
	receive(t, c, "<r 29 -1>")
	if c.Snapshot().CV29 != 227 {
		t.Fatal("failure changed CV29")
	}
	receive(t, c, "<r 300>")
	if c.Snapshot().ProgramAddress != "300" {
		t.Fatal("address read")
	}
	receive(t, c, "<w 301>")
	if c.Snapshot().ProgramResult != "address written: 301" {
		t.Fatal("address write")
	}
	s.commands = nil
	c.ReadAddress()
	c.WriteAddress(300)
	c.ReadCV(3)
	c.WriteCV(3, 20)
	c.POM(3, 10)
	want := []string{"<R>", "<W 300>", "<R 3>", "<W 3 20>", "<w 3 3 10>"}
	if !reflect.DeepEqual(s.commands, want) {
		t.Fatal(s.commands)
	}
}
func TestCloseAndSnapshot(t *testing.T) {
	c, s := setup(t)
	c.Power(true, p.Main)
	c.Raw(" D CABS ")
	c.Close()
	if !reflect.DeepEqual(s.commands, []string{"<1 MAIN>", "<D CABS>", "<0>"}) || !s.closed {
		t.Fatal("close", s.commands)
	}
	for i := 0; i < 600; i++ {
		c.Log("info", "hello")
	}
	snap := c.Snapshot()
	if len(snap.Logs) != 500 {
		t.Fatal("unbounded log")
	}
	snap.Logs[0].Text = "mutated"
	if c.Snapshot().Logs[0].Text == "mutated" {
		t.Fatal("snapshot aliases controller")
	}
}
