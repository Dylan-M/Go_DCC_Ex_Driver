package fyneui

import (
	"fyne.io/fyne/v2/test"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"testing"
)

type powerTestSender struct{}

func (powerTestSender) Send(string) error { return nil }
func (powerTestSender) Close() error      { return nil }

func TestPersistentPowerIndicators(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("power test")
	session := th.NewSession(config.Default(), nil)
	t.Cleanup(func() { session.Close(); <-session.Done(); w.Close() })
	v := New(w, session)
	c := th.New(config.Default().Toggle)
	if err := c.Attach(powerTestSender{}, "test"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ frame, main, prog string }{
		{"<p0>", "Main (Off)", "Prog (Off)"},
		{"<p1 MAIN>", "Main (On)", "Prog (Off)"},
		{"<p1 PROG>", "Main (On)", "Prog (On)"},
		{"<p0 MAIN>", "Main (Off)", "Prog (On)"},
		{"<p2 PROG>", "Main (Off)", "Prog (Overload)"},
	} {
		e, err := p.Parse(step.frame)
		if err != nil {
			t.Fatal(err)
		}
		c.Receive(e)
		v.Render(c.Snapshot())
		if v.mainPower.Text != step.main || v.progPower.Text != step.prog {
			t.Fatalf("%s: %s / %s", step.frame, v.mainPower.Text, v.progPower.Text)
		}
		if !v.mainPower.Visible() || !v.progPower.Visible() {
			t.Fatal("indicators must remain visible")
		}
	}
	c.Detach("lost connection")
	v.Render(c.Snapshot())
	if v.mainPower.Text != "Main (Unknown)" || v.progPower.Text != "Prog (Unknown)" {
		t.Fatal("stale disconnected display")
	}
}
