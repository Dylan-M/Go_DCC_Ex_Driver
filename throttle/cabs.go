package throttle

import (
	"errors"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"slices"
	"time"
)

type CabState struct {
	Cab, Speed, Direction int
	Functions             [29]bool
	Name                  string
}

type cabRuntime struct {
	state                    CabState
	pending                  *int
	lastSpeed, lastDirection int
	lastKnown                bool
	lastSent                 time.Time
}

func (c *Controller) currentCab() cabRuntime {
	r := c.cabs[c.state.Cab]
	r.state.Cab, r.state.Speed, r.state.Direction, r.state.Functions = c.state.Cab, c.state.Speed, c.state.Direction, c.state.Functions
	r.pending, r.lastSpeed, r.lastDirection, r.lastKnown, r.lastSent = c.pending, c.lastSpeed, c.lastDirection, c.lastKnown, c.lastSent
	return r
}
func (c *Controller) storeCab() { c.cabs[c.state.Cab] = c.currentCab() }
func (c *Controller) loadCab(cab int) {
	r := c.cabs[cab]
	c.state.Cab, c.state.Speed, c.state.Direction, c.state.Functions = r.state.Cab, r.state.Speed, r.state.Direction, r.state.Functions
	c.pending, c.lastSpeed, c.lastDirection, c.lastKnown, c.lastSent = r.pending, r.lastSpeed, r.lastDirection, r.lastKnown, r.lastSent
}
func (c *Controller) cabStates() []CabState {
	result := make([]CabState, 0, len(c.cabs))
	for _, cab := range c.order {
		r := c.cabs[cab]
		if cab == c.state.Cab {
			r = c.currentCab()
		}
		result = append(result, r.state)
	}
	return result
}

// WithCab routes queued UI intents to the originating throttle, even if focus
// changes before the session processes them. It never changes the active tab.
func (c *Controller) WithCab(cab int, fn func(*Controller) error) error {
	if _, ok := c.cabs[cab]; !ok {
		return errors.New("throttle has been closed")
	}
	active := c.state.Cab
	c.storeCab()
	c.loadCab(cab)
	err := fn(c)
	c.storeCab()
	c.loadCab(active)
	return err
}

func (c *Controller) AddCab(cab int) error {
	if _, ok := c.cabs[cab]; ok {
		return errors.New("a throttle for that address is already open")
	}
	if _, err := p.EncodeLocoRequest(cab); err != nil {
		return err
	}
	c.cabs[cab] = cabRuntime{state: CabState{Cab: cab, Direction: 1}}
	c.order = append(c.order, cab)
	return c.FocusCab(cab)
}

// FocusCab switches visible tabs without canceling another tab's speed intent.
// SelectCab retains its existing cancel-on-cab-change behavior for callers that
// are selecting a different locomotive rather than navigating open throttles.
func (c *Controller) FocusCab(cab int) error {
	if _, ok := c.cabs[cab]; !ok {
		return errors.New("throttle has been closed")
	}
	return c.selectCab(cab, true)
}

func (c *Controller) RemoveCab(cab int) error {
	c.storeCab()
	r, ok := c.cabs[cab]
	if !ok {
		return errors.New("throttle has been closed")
	}
	if len(c.cabs) == 1 {
		return errors.New("keep at least one throttle open")
	}
	if r.state.Speed != 0 || r.pending != nil {
		return errors.New("stop this locomotive before closing or reassigning its throttle")
	}
	delete(c.cabs, cab)
	c.order = slices.DeleteFunc(c.order, func(address int) bool { return address == cab })
	if c.state.Cab == cab {
		for _, other := range c.cabStates() {
			c.loadCab(other.Cab)
			break
		}
	}
	return nil
}

func (c *Controller) ReplaceCab(old, next int) error {
	if old == next {
		return nil
	}
	if _, err := p.EncodeLocoRequest(next); err != nil {
		return err
	}
	if _, ok := c.cabs[next]; ok {
		return errors.New("a throttle for that address is already open")
	}
	c.storeCab()
	r, ok := c.cabs[old]
	if !ok {
		return errors.New("throttle has been closed")
	}
	if r.state.Speed != 0 || r.pending != nil {
		return errors.New("stop this locomotive before reassigning its throttle")
	}
	// Keep the tab's position, including when it is the only open throttle.
	c.cabs[next] = cabRuntime{state: CabState{Cab: next, Direction: 1, Name: r.state.Name}}
	for i, address := range c.order {
		if address == old {
			c.order[i] = next
		}
	}
	err := c.FocusCab(next)
	delete(c.cabs, old)
	return err
}

func (c *Controller) Tick(now time.Time) error {
	for _, cab := range c.cabStates() {
		if err := c.WithCab(cab.Cab, func(c *Controller) error { return c.tickCurrent(now) }); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) receiveLoco(v p.LocoState) {
	c.state.Speed, c.state.Direction = v.Speed, v.Direction
	c.pending = nil
	c.lastSpeed = v.Speed
	c.lastDirection = v.Direction
	c.lastKnown = true
	for n := range c.state.Functions {
		c.state.Functions[n] = v.FunctionMask&(1<<n) != 0
	}
}
