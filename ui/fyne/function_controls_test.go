package fyneui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

type buttonSender struct{ commands chan string }

func (s buttonSender) Send(command string) error { s.commands <- command; return nil }
func (s buttonSender) Close() error              { return nil }

func TestFunctionControlsRoutePressReleaseAndAllOff(t *testing.T) {
	v, s := setupView(t)
	station := buttonSender{make(chan string, 32)}
	if err := s.Post(func(c *th.Controller) error {
		if err := c.AddCab(7); err != nil {
			return err
		}
		return c.Attach(station, "test")
	}); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return state.Connected })
	expect := func(want string) {
		t.Helper()
		select {
		case got := <-station.commands:
			if got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("missing command", want)
		}
	}
	for _, command := range []string{"<=>", "<s>", "<t 3>", "<t 7>"} {
		expect(command)
	}
	panel := v.panels[3]
	panel.functions[0].down()
	panel.functions[0].up()
	expect("<F 3 0 1>")
	expect("<F 3 0 0>")
	if err := s.Post(func(c *th.Controller) error {
		c.Receive(p.LocoState{Cab: 3, Direction: 1, FunctionMask: 1})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Functions[0] })
	var find func(fyne.CanvasObject) *widget.Button
	find = func(object fyne.CanvasObject) *widget.Button {
		switch x := object.(type) {
		case *widget.Button:
			if x.Text == "All Functions Off" {
				return x
			}
		case *container.Scroll:
			return find(x.Content)
		case *fyne.Container:
			for _, child := range x.Objects {
				if found := find(child); found != nil {
					return found
				}
			}
		}
		return nil
	}
	button := find(panel.tab.Content)
	if button == nil {
		t.Fatal("All Functions Off missing")
	}
	test.Tap(button)
	expect("<F 3 0 0>")
}
