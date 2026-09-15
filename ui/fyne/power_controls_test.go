package fyneui

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

// Drive real Fyne buttons through the production session/client. This peer
// supplies controlled replies so the test can distinguish intent from ACK.
func TestPowerButtonsFollowStationReplies(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("power controls")
	conn, peer := net.Pipe()
	commands := make(chan string, 64)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scan := bufio.NewScanner(peer)
		for scan.Scan() {
			commands <- scan.Text()
		}
	}()
	s := th.NewSession(config.Default(), func(context.Context, th.Connection) (io.ReadWriteCloser, error) { return conn, nil }, nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); peer.Close(); <-readerDone; w.Close() })
	v := New(w, s)
	if !v.mainPower.Disabled() || !v.progPower.Disabled() || !v.allOn.Disabled() || !v.allOff.Disabled() {
		t.Fatal("disconnected power controls enabled")
	}
	waitState := func(main, prog string) {
		t.Helper()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case state, ok := <-s.Updates():
				if !ok {
					t.Fatal("session ended")
				}
				v.Render(state)
				if v.mainPower.Text == main && v.progPower.Text == prog {
					return
				}
			case <-deadline.C:
				t.Fatalf("display timeout: %s / %s", v.mainPower.Text, v.progPower.Text)
			}
		}
	}
	expectCommand := func(want string) {
		t.Helper()
		select {
		case got := <-commands:
			if got != want {
				t.Fatalf("command %q; want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("command timeout", want)
		}
	}
	reply := func(frame string) {
		t.Helper()
		peer.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := io.WriteString(peer, frame+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	// Disable periodic queries to make command ordering deterministic.
	if err := s.Post(func(c *th.Controller) error { c.SetPoll(false); return nil }); err != nil {
		t.Fatal(err)
	}
	if err := s.Connect(th.Connection{Host: "test"}); err != nil {
		t.Fatal(err)
	}
	expectCommand("<=>")
	expectCommand("<s>")
	expectCommand("<t 3>")
	reply("<p0>")
	waitState("MAIN: OFF", "PROG: OFF")
	for _, step := range []struct {
		button                   *widget.Button
		command, ack, main, prog string
	}{
		{v.mainPower, "<1 MAIN>", "<p1 MAIN>", "MAIN: ON", "PROG: OFF"},
		{v.allOn, "<1>", "<p1>", "MAIN: ON", "PROG: ON"},
		{v.progPower, "<0 PROG>", "<p0 PROG>", "MAIN: ON", "PROG: OFF"},
		{v.mainPower, "<0 MAIN>", "<p0 MAIN>", "MAIN: OFF", "PROG: OFF"},
		{v.progPower, "<1 PROG>", "<p1 PROG>", "MAIN: OFF", "PROG: ON"},
		{v.allOff, "<0>", "<p0>", "MAIN: OFF", "PROG: OFF"},
	} {
		beforeMain, beforeProg := v.mainPower.Text, v.progPower.Text
		test.Tap(step.button)
		expectCommand(step.command)
		if v.mainPower.Text != beforeMain || v.progPower.Text != beforeProg {
			t.Fatal("optimistic power display before station reply")
		}
		reply(step.ack)
		waitState(step.main, step.prog)
	}
	reply("<p1 MAIN>") // Also reflect another throttle's changes.
	waitState("MAIN: ON", "PROG: OFF")
	if v.mainPower.Importance != widget.HighImportance || v.progPower.Importance == widget.HighImportance {
		t.Fatal("ON/OFF styling")
	}
	peer.Close()
	waitState("MAIN: UNKNOWN", "PROG: UNKNOWN")
	if !v.mainPower.Disabled() || !v.progPower.Disabled() || !v.allOn.Disabled() || !v.allOff.Disabled() {
		t.Fatal("disconnected controls enabled")
	}
}
