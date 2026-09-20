package fyneui

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func timingRecords(t *testing.T, d *tabTiming, output *bytes.Buffer) []tabTimingRecord {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	var records []tabTimingRecord
	for {
		var record tabTimingRecord
		if err := decoder.Decode(&record); errors.Is(err, io.EOF) {
			return records
		} else if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
}

func TestTabTimingStages(t *testing.T) {
	var output bytes.Buffer
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	d := &tabTiming{output: &output, now: func() time.Time { return now }, delay: 500 * time.Millisecond}
	d.input(7, "pointer_down")
	now = now.Add(80 * time.Millisecond)
	d.input(7, "pointer_up")
	now = now.Add(500 * time.Millisecond)
	d.input(7, "tap_dispatched")
	s := d.span(7, false)
	if s == nil || d.span(3, false) != nil || d.span(7, true) != nil {
		t.Fatal("wrong trace correlation")
	}
	s.mark("selection_begin")
	now = now.Add(10 * time.Millisecond)
	s.mark("selection_layout_complete")
	s.mark("session_focus_queued")
	now = now.Add(15 * time.Millisecond)
	s.mark("session_focus_begin")
	now = now.Add(5 * time.Millisecond)
	s.mark("session_focus_complete")
	if d.span(7, false) != nil || d.span(7, true) != s {
		t.Fatal("render phase not separated from selection phase")
	}
	now = now.Add(30 * time.Millisecond)
	s.mark("state_render_begin")
	now = now.Add(10 * time.Millisecond)
	s.mark("state_render_complete")
	s.mark("must_not_log_after_completion")
	if d.span(7, true) != nil {
		t.Fatal("completed trace remained active")
	}
	records := timingRecords(t, d, &output)
	if len(records) != 10 {
		t.Fatal(records)
	}
	if records[2].ElapsedMS != 580 || records[2].SinceLastMS != 500 || records[9].ElapsedMS != 650 || records[9].SinceLastMS != 10 {
		t.Fatal("elapsed or stage timing incorrect", records)
	}
	for _, record := range records {
		if record.Event != "tab_timing" || record.ID != 1 || record.Cab != 7 || record.DoubleClickMS != 500 {
			t.Fatal(record)
		}
	}
}

func TestTabTimingAbandonedAndDoubleClicks(t *testing.T) {
	var output bytes.Buffer
	now := time.Now()
	d := &tabTiming{output: &output, now: func() time.Time { return now }, delay: 500 * time.Millisecond}
	d.input(3, "pointer_up") // No press to correlate.
	d.input(3, "pointer_down")
	first := d.current
	if d.span(3, false) != nil {
		t.Fatal("unresolved click exposed to background selection updates")
	}
	d.input(7, "pointer_up") // An unrelated target must not extend this gesture.
	d.input(7, "pointer_down")
	first.mark("late_callback")
	now = now.Add(50 * time.Millisecond)
	d.input(7, "pointer_up")
	now = now.Add(100 * time.Millisecond)
	d.input(7, "pointer_down")
	if d.current.id != 2 {
		t.Fatal("second press lost first-click timing")
	}
	d.input(7, "double_tap_dispatched")
	d.input(7, "tap_dispatched")
	d.input(7, "pointer_down")
	d.input(7, "pointer_up")
	now = now.Add(time.Second)
	d.input(7, "pointer_down")
	d.input(7, "long_hold_dispatched")
	d.input(7, "pointer_down")
	d.input(7, "tap_dispatched")
	d.span(7, false).mark("selection_unchanged")
	for _, record := range timingRecords(t, d, &output) {
		if record.Stage == "late_callback" {
			t.Fatal("superseded trace recorded a later callback")
		}
	}
}

type failedTimingWriter struct{}

func (failedTimingWriter) Write([]byte) (int, error) {
	return 0, errors.New("diagnostic output failed")
}

func TestTabTimingDisabledAndOutputFailure(t *testing.T) {
	t.Setenv("DCCEX_DEBUG_TAB_TIMING", "0")
	if tabTimingFromEnvironment() != nil {
		t.Fatal("timing must be opt-in")
	}
	test.NewTempApp(t)
	t.Setenv("DCCEX_DEBUG_TAB_TIMING", "1")
	if tabTimingFromEnvironment() == nil {
		t.Fatal("timing environment option ignored")
	}
	var disabled *tabTiming
	disabled.input(3, "pointer_down")
	disabled.span(3, false).mark("disabled")
	d := &tabTiming{output: failedTimingWriter{}, now: time.Now}
	d.input(3, "pointer_down")
	if d.output != nil {
		t.Fatal("broken diagnostic output was not disabled")
	}
	d.input(7, "pointer_down")
	d.current.mark("ignored")
	d.current.writeLocked("ignored", time.Now())
}

func TestTabTimingConcurrentCallbacks(t *testing.T) {
	var output bytes.Buffer
	d := &tabTiming{output: &output, now: time.Now}
	d.input(3, "pointer_down")
	d.input(3, "tap_dispatched")
	s := d.span(3, false)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			for range 20 {
				s.mark("concurrent_stage")
				d.span(3, false)
			}
		})
	}
	group.Wait()
	if len(timingRecords(t, d, &output)) != 162 {
		t.Fatal("concurrent log records were lost")
	}
}

func TestTabTimingViewIntegration(t *testing.T) {
	v, s := setupView(t)
	var output bytes.Buffer
	d := &tabTiming{output: &output, now: time.Now, delay: 500 * time.Millisecond}
	v.tabTiming, v.runTabs.timing = d, d
	if err := s.Post(func(c *th.Controller) error { return c.AddCab(7) }); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return state.Cab == 7 })
	button := v.runTabs.headers[v.panels[3].tab].title
	button.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonSecondary})
	button.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonSecondary})
	if output.Len() != 0 {
		t.Fatal("right click started a selection trace")
	}
	button.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	if v.runTabs.Selected() != v.panels[7].tab {
		t.Fatal("diagnostics changed selection-on-press behavior")
	}
	button.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	test.Tap(button)
	renderUntil(t, v, s, func(state th.State) bool { return state.Cab == 3 })
	records := timingRecords(t, d, &output)
	stages := make(map[string]bool)
	for _, record := range records {
		stages[record.Stage] = true
	}
	for _, stage := range []string{"pointer_down", "pointer_up", "tap_dispatched", "selection_begin", "selection_layout_complete", "session_focus_queued", "session_focus_begin", "session_focus_complete", "selection_callback_complete", "state_render_begin", "state_render_complete"} {
		if !stages[stage] {
			t.Fatal("missing stage", stage, records)
		}
	}
	if stages["selection_unchanged"] || records[len(records)-1].Stage != "state_render_complete" {
		t.Fatal("snapshot confirmation ended timing before rendering completed", records)
	}
	if strings.Contains(output.String(), "Locomotive") || v.runTabs.cab(nil) != 0 {
		t.Fatal("unexpected name logging or unknown-tab identity")
	}
	// A failed queue must not leave a trace waiting for a nonexistent update.
	s.Close()
	<-s.Done()
	d.input(7, "pointer_down")
	d.input(7, "tap_dispatched")
	v.runTabs.OnSelected(v.panels[7].tab)
	records = timingRecords(t, d, &output)
	if records[len(records)-1].Stage != "session_queue_error" {
		t.Fatal("queue failure not recorded")
	}
}

func TestTabTimingControllerError(t *testing.T) {
	v, s := setupView(t)
	var output bytes.Buffer
	d := &tabTiming{output: &output, now: time.Now}
	v.tabTiming, v.runTabs.timing = d, d
	if err := s.Post(func(c *th.Controller) error { return c.AddCab(7) }); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return state.Cab == 7 })
	// Keep a stale view while the controller removes its old address.
	if err := s.Post(func(c *th.Controller) error { return c.RemoveCab(3) }); err != nil {
		t.Fatal(err)
	}
	d.input(3, "pointer_down")
	d.input(3, "tap_dispatched")
	v.runTabs.OnSelected(v.panels[3].tab)
	barrier := make(chan struct{})
	if err := s.Post(func(*th.Controller) error { close(barrier); return nil }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-barrier:
	case <-time.After(5 * time.Second):
		t.Fatal("session action timeout")
	}
	records := timingRecords(t, d, &output)
	if records[len(records)-1].Stage != "session_focus_error" {
		t.Fatal("controller error not measured", records)
	}
}
