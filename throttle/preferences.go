package throttle

import (
	"errors"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"strings"
)

// TabPersistence is optional for embedders. The application supplies storage
// appropriate to its platform; the session never assumes a desktop filesystem.
type TabPersistence struct {
	Initial config.ThrottleSettings
	Save    func(config.ThrottleSettings) error
}

func (c *Controller) restoreTabs(settings config.ThrottleSettings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	c.cabs = make(map[int]cabRuntime, len(settings.Tabs))
	c.order = make([]int, 0, len(settings.Tabs))
	for _, tab := range settings.Tabs {
		toggle := c.defaultToggle
		if tab.Toggle != nil {
			toggle = *tab.Toggle
		}
		c.cabs[tab.Address] = cabRuntime{state: CabState{Cab: tab.Address, Direction: 1, Name: tab.Name, Labels: tab.Labels, Toggle: toggle}}
		c.order = append(c.order, tab.Address)
	}
	c.loadCab(settings.Selected)
	return nil
}

func (c *Controller) tabSettings() config.ThrottleSettings {
	s := config.ThrottleSettings{Version: 1, Selected: c.state.Cab}
	for _, cab := range c.cabStates() {
		toggle := cab.Toggle
		s.Tabs = append(s.Tabs, config.ThrottleTab{Address: cab.Cab, Name: cab.Name, Labels: cab.Labels, Toggle: &toggle})
	}
	return s
}

// RenameCab changes a local tab preference without sending operating commands.
func (c *Controller) RenameCab(cab int, name string) error {
	r, ok := c.cabs[cab]
	if !ok {
		return errors.New("throttle has been closed")
	}
	if err := config.ValidateDisplayName(name); err != nil {
		return err
	}
	r.state.Name = strings.TrimSpace(name)
	c.cabs[cab] = r
	return nil
}

// SetFunctionLabel changes only this tab's presentation, not its function state.
func (c *Controller) SetFunctionLabel(cab, n int, label string) error {
	r, ok := c.cabs[cab]
	if !ok {
		return errors.New("throttle has been closed")
	}
	if !validFunction(n) {
		return errors.New("function must be F0-F28")
	}
	if err := config.ValidateDisplayName(label); err != nil {
		return err
	}
	r.state.Labels[n] = strings.TrimSpace(label)
	c.cabs[cab] = r
	return nil
}

// SetFunctionLabel queues a local edit that is flushed before session shutdown.
func (s *Session) SetFunctionLabel(cab, n int, label string) error {
	return s.enqueue(sessionAction{preference: true, apply: func(c *Controller) error { return c.SetFunctionLabel(cab, n, label) }})
}

// SetFunctionToggle preserves the originating cab and survives session close.
func (s *Session) SetFunctionToggle(cab, n int, on bool) error {
	return s.enqueue(sessionAction{preference: true, apply: func(c *Controller) error {
		return c.WithCab(cab, func(c *Controller) error { return c.SetToggle(n, on) })
	}})
}

// FlipFunctionToggle reads the mode when the intent executes, not from a
// potentially stale UI snapshot when several flips are queued together.
func (s *Session) FlipFunctionToggle(cab, n int) error {
	return s.enqueue(sessionAction{preference: true, apply: func(c *Controller) error {
		if !validFunction(n) {
			return errors.New("function must be F0-F28")
		}
		return c.WithCab(cab, func(c *Controller) error { return c.SetToggle(n, !c.state.Toggle[n]) })
	}})
}
