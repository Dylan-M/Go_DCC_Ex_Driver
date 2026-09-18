package throttle

import "github.com/Dylan-M/Go_DCC_Ex_Driver/config"

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
		c.cabs[tab.Address] = cabRuntime{state: CabState{Cab: tab.Address, Direction: 1}}
		c.order = append(c.order, tab.Address)
	}
	c.loadCab(settings.Selected)
	return nil
}

func (c *Controller) tabSettings() config.ThrottleSettings {
	s := config.ThrottleSettings{Version: 1, Selected: c.state.Cab}
	for _, address := range c.order {
		s.Tabs = append(s.Tabs, config.ThrottleTab{Address: address})
	}
	return s
}
