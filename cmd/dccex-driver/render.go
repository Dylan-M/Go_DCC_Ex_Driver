package main

import (
	"sync"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

// Start consuming snapshots only after Fyne initializes its UI event queue.
// The mobile driver executes dispatched functions inline before that point,
// which would race window initialization. Repeated lifecycle events are safe.
func stateRenderer(updates <-chan throttle.State, render func(throttle.State), dispatch func(func())) func() {
	return sync.OnceFunc(func() {
		go func() {
			for state := range updates {
				dispatch(func() { render(state) })
			}
		}()
	})
}
