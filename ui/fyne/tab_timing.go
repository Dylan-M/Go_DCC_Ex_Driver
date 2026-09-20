package fyneui

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// tabTiming is opt-in and independent of controller logs, so measuring a tab
// switch does not itself publish more state snapshots or repaint the console.
// UI and session goroutines share it; all span state and writes use mu.
type tabTiming struct {
	mu      sync.Mutex
	output  io.Writer
	now     func() time.Time
	delay   time.Duration
	next    uint64
	current *tabTimingSpan
}

type tabTimingSpan struct {
	owner                 *tabTiming
	id                    uint64
	cab                   int
	start, last, released time.Time
	waiting, ready, done  bool
}

type tabTimingRecord struct {
	Event         string    `json:"event"`
	ID            uint64    `json:"id"`
	Cab           int       `json:"cab"`
	Stage         string    `json:"stage"`
	At            time.Time `json:"at"`
	ElapsedMS     float64   `json:"elapsed_ms"`
	SinceLastMS   float64   `json:"since_previous_ms"`
	DoubleClickMS float64   `json:"double_click_ms"`
}

func tabTimingFromEnvironment() *tabTiming {
	if os.Getenv("DCCEX_DEBUG_TAB_TIMING") != "1" {
		return nil
	}
	return &tabTiming{output: os.Stderr, now: time.Now, delay: fyne.CurrentApp().Driver().DoubleTapDelay()}
}

func (d *tabTiming) input(cab int, stage string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.output == nil {
		return
	}
	now := d.now()
	s := d.current
	if stage == "pointer_down" {
		// Preserve the first press when this is the second click of the
		// same unresolved gesture. An abandoned gesture gets its own end.
		if s == nil || s.done || s.cab != cab || !s.waiting || s.released.IsZero() || now.Sub(s.released) > d.delay {
			if s != nil && !s.done {
				s.writeLocked("superseded", now)
				s.done = true
			}
			d.next++
			s = &tabTimingSpan{owner: d, id: d.next, cab: cab, start: now, last: now, waiting: true}
			d.current = s
		}
	}
	if s == nil || s.done || s.cab != cab {
		return
	}
	if stage == "pointer_up" {
		s.released = now
	}
	if stage == "tap_dispatched" {
		s.waiting = false
	}
	s.writeLocked(stage, now)
	if stage == "double_tap_dispatched" || stage == "long_hold_dispatched" {
		// Editing is not a tab-load measurement. Keep its gesture timing
		// separate even though the existing editor also selects its tab.
		s.done = true
	}
}

func (d *tabTiming) span(cab int, requireReady bool) *tabTimingSpan {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.current
	// Background snapshots can reselect the current tab while a click is
	// still being classified. They must not end that unresolved gesture.
	if s == nil || s.done || s.waiting || s.cab != cab || s.ready != requireReady {
		return nil
	}
	return s
}

func (s *tabTimingSpan) mark(stage string) {
	if s == nil {
		return
	}
	d := s.owner
	d.mu.Lock()
	defer d.mu.Unlock()
	if s.done || d.output == nil {
		return
	}
	s.writeLocked(stage, d.now())
	switch stage {
	case "session_focus_complete", "session_focus_error":
		s.ready = true
	case "state_render_complete", "selection_unchanged", "session_queue_error":
		s.done = true
	}
}

func (s *tabTimingSpan) writeLocked(stage string, now time.Time) {
	d := s.owner
	if d.output == nil {
		return
	}
	record := tabTimingRecord{Event: "tab_timing", ID: s.id, Cab: s.cab, Stage: stage, At: now.UTC(),
		ElapsedMS:     float64(now.Sub(s.start)) / float64(time.Millisecond),
		SinceLastMS:   float64(now.Sub(s.last)) / float64(time.Millisecond),
		DoubleClickMS: float64(d.delay) / float64(time.Millisecond)}
	s.last = now
	if err := json.NewEncoder(d.output).Encode(record); err != nil {
		// Diagnostics must not interrupt train control or retry a broken
		// output on every UI event. Report once and disable further writes.
		d.output = nil
		log.Printf("tab timing disabled: %v", err)
	}
}
