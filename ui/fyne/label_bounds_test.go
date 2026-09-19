package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"strings"
	"testing"
)

func TestLongLabelsDoNotWidenWindow(t *testing.T) {
	v, s := setupView(t)
	panel := v.panels[3]
	test.Tap(panel.setupButton)
	long := strings.Repeat("W", 80)
	panel.labels[0].SetText(long)
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Labels[0] == long })
	button := panel.functions[0]
	if !strings.HasSuffix(button.Text, "…") || button.AccessibilityLabel() != long {
		t.Fatal(button.Text, button.AccessibilityLabel())
	}
	if width := v.Window.Content().MinSize().Width; width > 1050 {
		t.Fatalf("long label widened window to %.0f", width)
	}
	panel.setup.Hide()
	panel.showSetup()
	if panel.labels[0].Text != long {
		t.Fatal("stored label was truncated")
	}
}

func TestButtonLabelBoundsKeepModeAndGraphemes(t *testing.T) {
	test.NewTempApp(t)
	b := newFunctionButton("F0", nil, nil, nil)
	if b.AccessibilityLabel() != "F0" {
		t.Fatal("fallback label")
	}
	for _, label := range []string{"Headlight", strings.Repeat("W", 80), strings.Repeat("界", 80), strings.Repeat("e\u0301", 40), strings.Repeat("👩‍🚒", 15)} {
		b.SetLabel(label, true)
		if !strings.HasSuffix(b.Text, " ↕") || b.AccessibilityLabel() != label+" ↕" {
			t.Fatal(b.Text)
		}
		if width := fyne.MeasureText(b.Text, theme.TextSize(), fyne.TextStyle{Bold: true}).Width; width > 112 {
			t.Fatalf("label width %.1f: %q", width, b.Text)
		}
	}
	cluster := "e\u0301"
	size := theme.TextSize()
	width := fyne.MeasureText(cluster+"…", size, fyne.TextStyle{Bold: true}).Width
	if got := shortFunctionLabel(strings.Repeat(cluster, 40), width, size); got != cluster+"…" {
		t.Fatalf("split grapheme: %q", got)
	}
	if got := shortFunctionLabel("W", 0, size); got != "…" {
		t.Fatal(got)
	}
}
