package main

import (
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestStateRendererWaitsForStartAndDispatchesInOrder(t *testing.T) {
	updates := make(chan throttle.State, 2)
	updates <- throttle.State{Cab: 3}
	updates <- throttle.State{Cab: 7}
	close(updates)
	rendered := make(chan int, 2)
	var onDispatch bool
	start := stateRenderer(updates, func(s throttle.State) {
		if !onDispatch {
			t.Error("render bypassed UI dispatcher")
		}
		rendered <- s.Cab
	}, func(f func()) { onDispatch = true; f(); onDispatch = false })
	if len(updates) != 2 || len(rendered) != 0 {
		t.Fatal("rendering started before lifecycle callback")
	}
	start()
	start()
	for _, want := range []int{3, 7} {
		select {
		case got := <-rendered:
			if got != want {
				t.Fatalf("got %d, want %d", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("render timeout")
		}
	}
}
