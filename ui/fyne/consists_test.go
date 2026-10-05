package fyneui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/capabilities"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestConsistDiscoveryStatus(t *testing.T) {
	for _, tc := range []struct {
		status capabilities.Status
		text   string
	}{
		{capabilities.NotRequested, "No discovery query has been sent"},
		{capabilities.Pending, "Waiting"},
		{capabilities.Available, "verification and builder implementation remain pending"},
		{capabilities.Unsupported, "rejected the capability query (<X>)"},
		{capabilities.Inconclusive, "Support is unknown"},
		{capabilities.InvalidResponse, "response was invalid"},
		{capabilities.Status(255), "support is unknown"},
	} {
		state := th.State{Connected: true, Discovery: capabilities.Snapshot{Status: tc.status}}
		if text := consistDiscoveryText(state); !strings.Contains(text, tc.text) {
			t.Fatalf("status %v: %s", tc.status, text)
		}
		state.Connected = false
		if text := consistDiscoveryText(state); !strings.HasPrefix(text, "Connect to a command station") {
			t.Fatal("disconnected view exposes stale capability state", text)
		}
	}
}

func TestConsistTabIsReadOnlyAndRendersStatus(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("consist foundation")
	s := th.NewSession(config.Default(), nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
	v := newView(w, s, false, Options{PowerThrottle: true})
	if len(v.tabs.Items) != 4 || v.tabs.Items[3].Text != "Consists" {
		t.Fatal("consist tab missing")
	}
	controls := 0
	walkControls(v.tabs.Items[3].Content, func(object fyne.CanvasObject) {
		switch object.(type) {
		case *widget.Button, *widget.Entry, *widget.Check, *widget.Select:
			controls++
		}
	})
	if controls != 0 {
		t.Fatal("scaffold exposes operating controls")
	}
	state := th.State{Connected: true, Discovery: capabilities.Snapshot{Status: capabilities.Unsupported}}
	v.Render(state)
	if v.consistStatus.Text != consistDiscoveryText(state) {
		t.Fatal("capability state not rendered")
	}
	state.Connected = false
	v.Render(state)
	if v.consistStatus.Text != consistDiscoveryText(state) {
		t.Fatal("disconnect retained stale message")
	}
}
