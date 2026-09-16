package fyneui

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"fyne.io/fyne/v2"
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
	waitState("Main (Off)", "Prog (Off)")
	for _, step := range []struct {
		button                   *widget.Button
		command, ack, main, prog string
	}{
		{v.mainPower, "<1 MAIN>", "<p1 MAIN>", "Main (On)", "Prog (Off)"},
		{v.allOn, "<1>", "<p1>", "Main (On)", "Prog (On)"},
		{v.progPower, "<0 PROG>", "<p0 PROG>", "Main (On)", "Prog (Off)"},
		{v.mainPower, "<0 MAIN>", "<p0 MAIN>", "Main (Off)", "Prog (Off)"},
		{v.progPower, "<1 PROG>", "<p1 PROG>", "Main (Off)", "Prog (On)"},
		{v.allOff, "<0>", "<p0>", "Main (Off)", "Prog (Off)"},
	} {
		beforeMain, beforeProg := v.mainPower.Text, v.progPower.Text
		beforeMainColor, beforeProgColor := v.mainPowerTheme.Theme, v.progPowerTheme.Theme
		test.Tap(step.button)
		expectCommand(step.command)
		if v.mainPower.Text != beforeMain || v.progPower.Text != beforeProg {
			t.Fatal("optimistic power display before station reply")
		}
		if v.mainPowerTheme.Theme != beforeMainColor || v.progPowerTheme.Theme != beforeProgColor {
			t.Fatal("optimistic power colors before station reply")
		}
		reply(step.ack)
		waitState(step.main, step.prog)
	}
	reply("<p1 MAIN>") // Also reflect another throttle's changes.
	waitState("Main (On)", "Prog (Off)")
	if v.mainPower.Importance != widget.HighImportance || v.progPower.Importance != widget.HighImportance {
		t.Fatal("ON/OFF styling")
	}
	// The two-position selector names a destination instead of toggling blindly.
	if v.direction.Selected != "Fwd" {
		t.Fatal("direction selector must show one current direction")
	}
	v.direction.Resize(fyne.NewSize(200, 36))
	waitDirection := func(want int) {
		t.Helper()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case state, ok := <-s.Updates():
				if !ok {
					t.Fatal("session ended before direction update")
				}
				v.Render(state)
				if state.Direction == want {
					return
				}
			case <-deadline.C:
				t.Fatal("direction update timed out")
			}
		}
	}
	assertNoCommand := func() {
		t.Helper()
		processed := make(chan struct{})
		if err := s.Post(func(*th.Controller) error { close(processed); return nil }); err != nil {
			t.Fatal(err)
		}
		select {
		case <-processed:
		case <-time.After(5 * time.Second):
			t.Fatal("session barrier timed out")
		}
		select {
		case cmd := <-commands:
			t.Fatal("unexpected direction command", cmd)
		default:
		}
	}
	test.TapAt(v.direction, fyne.NewPos(25, 18)) // Rev
	expectCommand("<t 3 0 0>")
	waitDirection(0)
	if v.direction.Selected != "Rev" {
		t.Fatal("reverse not selected")
	}
	test.TapAt(v.direction, fyne.NewPos(25, 18)) // Already selected: no toggle or deselection.
	assertNoCommand()
	if v.direction.Selected != "Rev" {
		t.Fatal("selected direction was cleared")
	}
	test.TapAt(v.direction, fyne.NewPos(175, 18)) // Fwd
	expectCommand("<t 3 0 1>")
	waitDirection(1)
	reply("<l 3 0 0 0>") // Another throttle selects reverse.
	waitDirection(0)
	if v.direction.Selected != "Rev" {
		t.Fatal("external direction change not displayed")
	}
	assertNoCommand() // Rendering a station update must not echo a command.
	peer.Close()
	waitState("Main (Unknown)", "Prog (Unknown)")
	if !v.mainPower.Disabled() || !v.progPower.Disabled() || !v.allOn.Disabled() || !v.allOff.Disabled() {
		t.Fatal("disconnected controls enabled")
	}
	if !v.direction.Disabled() {
		t.Fatal("disconnected direction selector enabled")
	}
}
