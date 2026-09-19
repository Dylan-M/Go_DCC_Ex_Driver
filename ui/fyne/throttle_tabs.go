package fyneui

import (
	"fmt"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"slices"
	"strconv"
)

func (v *View) syncThrottles(s th.State) {
	// Legacy/simple state fixtures can still describe one throttle directly.
	cabs := s.Throttles
	if len(cabs) == 0 {
		cabs = []th.CabState{{Cab: s.Cab, Speed: s.Speed, Direction: s.Direction, Functions: s.Functions}}
	}
	wanted := make(map[int]bool, len(cabs))
	items := make([]*container.TabItem, 0, len(cabs))
	for _, cab := range cabs {
		wanted[cab.Cab] = true
		panel := v.panels[cab.Cab]
		if panel == nil {
			panel = &throttlePanel{owner: v, address: cab.Cab}
			panel.tab = container.NewTabItem(fmt.Sprintf("Loco %d", cab.Cab), panel.build())
			v.panels[cab.Cab] = panel
		}
		title := cab.Name
		if title == "" {
			title = fmt.Sprintf("Loco %d", cab.Cab)
		}
		if panel.tab.Text != title {
			panel.tab.Text = title
			v.runTabs.Refresh()
		}
		items = append(items, panel.tab)
		panel.render(s, cab)
	}
	for cab, panel := range v.panels {
		if !wanted[cab] {
			if panel.setup != nil {
				panel.setup.Hide()
			}
			for _, button := range panel.functions {
				button.up()
			}
			delete(v.panels, cab)
		}
	}
	if !slices.Equal(v.runTabs.Items, items) {
		v.runTabs.SetItems(items)
	}
	if panel := v.panels[s.Cab]; panel != nil {
		v.throttlePanel = panel
		v.runTabs.Select(panel.tab)
	}
}

func (v *View) addThrottleDialog() {
	address := entry("")
	dialog.ShowForm("Add throttle", "Add", "Cancel", []*widget.FormItem{widget.NewFormItem("Locomotive address", address)}, func(ok bool) {
		if !ok {
			return
		}
		cab, err := strconv.Atoi(address.Text)
		if err != nil {
			v.showError(fmt.Errorf("enter a locomotive address from 1 to 10293"))
			return
		}
		v.post(func(c *th.Controller) error { return c.AddCab(cab) })
	}, v.Window)
}
