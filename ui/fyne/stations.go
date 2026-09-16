package fyneui

import (
	"errors"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
)

func (v *View) stationControls() fyne.CanvasObject {
	v.savedStations = widget.NewSelect(nil, func(name string) {
		v.deleteStation.Disable()
		for _, p := range v.profiles {
			if p.Name != name {
				continue
			}
			v.mode.SetSelected(p.Mode)
			if p.Mode == "TCP" {
				v.host.SetText(p.Host)
				v.port.SetText(strconv.Itoa(p.Port))
			} else {
				v.devices.SetText(p.Device)
				v.baud.SetText(strconv.Itoa(p.Baud))
			}
			v.deleteStation.Enable()
			break
		}
	})
	v.savedStations.PlaceHolder = "Saved stations"
	v.saveStation = widget.NewButton("Save…", func() {
		name := entry(v.savedStations.Selected)
		dialog.ShowForm("Save command station", "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", name)}, func(ok bool) {
			if !ok {
				return
			}
			p, err := v.stationProfile(name.Text)
			if err != nil {
				v.showError(err)
				return
			}
			err = v.persistStation(p, false)
			if errors.Is(err, stations.ErrExists) {
				dialog.ShowConfirm("Replace saved station?", "Replace the saved connection named "+p.Name+"?", func(yes bool) {
					if yes {
						v.showError(v.persistStation(p, true))
					}
				}, v.Window)
			} else {
				v.showError(err)
			}
		}, v.Window)
	})
	v.deleteStation = widget.NewButton("Delete…", func() {
		name := v.savedStations.Selected
		if name == "" {
			return
		}
		dialog.ShowConfirm("Delete saved station?", "Remove "+name+" from saved stations? This does not disconnect or alter the command station.", func(yes bool) {
			if yes {
				v.showError(v.removeStation(name))
			}
		}, v.Window)
	})
	v.deleteStation.Disable()
	if v.stationStore == nil {
		v.savedStations.PlaceHolder = "Saved stations unavailable"
		v.savedStations.Disable()
		v.saveStation.Disable()
	} else if err := v.reloadStations(""); err != nil {
		v.savedStations.Disable()
		v.saveStation.Disable()
		v.savedStations.PlaceHolder = "Saved stations unavailable"
		v.showError(err)
	}
	return container.NewBorder(nil, nil, widget.NewLabel("Station"), container.NewHBox(v.saveStation, v.deleteStation), v.savedStations)
}

func (v *View) stationProfile(name string) (stations.Profile, error) {
	p := stations.Profile{Name: name, Mode: v.mode.Selected, Host: v.host.Text, Device: v.devices.Text}
	var err error
	if p.Mode == "TCP" {
		p.Port, err = strconv.Atoi(v.port.Text)
	} else {
		p.Baud, err = strconv.Atoi(v.baud.Text)
	}
	if err != nil {
		return p, errors.New("enter a valid numeric port or baud rate before saving")
	}
	return stations.Normalize(p)
}

func (v *View) reloadStations(selected string) error {
	list, err := v.stationStore.List()
	if err != nil {
		return err
	}
	v.profiles = list
	names := make([]string, len(list))
	for i, p := range list {
		names[i] = p.Name
	}
	// Updating the list must not silently load a profile over CLI/current fields.
	callback := v.savedStations.OnChanged
	v.savedStations.OnChanged = nil
	v.savedStations.SetOptions(names)
	if selected == "" {
		v.savedStations.ClearSelected()
	} else {
		v.savedStations.SetSelected(selected)
	}
	v.savedStations.OnChanged = callback
	if selected == "" {
		v.deleteStation.Disable()
	} else {
		v.deleteStation.Enable()
	}
	return nil
}

func (v *View) persistStation(p stations.Profile, replace bool) error {
	if v.stationStore == nil {
		return errors.New("saved stations unavailable")
	}
	if err := v.stationStore.Save(p, replace); err != nil {
		return err
	}
	return v.reloadStations(p.Name)
}

func (v *View) removeStation(name string) error {
	if v.stationStore == nil {
		return errors.New("saved stations unavailable")
	}
	if err := v.stationStore.Delete(name); err != nil {
		return err
	}
	if err := v.reloadStations(""); err != nil {
		return err
	}
	if v.last.Connected {
		// Restore the endpoint actually opened by the session, including CLI
		// launches. The currently selected saved profile may be unrelated.
		active := v.last.ActiveConnection
		if active.Serial {
			v.mode.SetSelected("Serial")
			v.devices.SetText(active.Device)
			v.baud.SetText(strconv.Itoa(active.Baud))
		} else {
			v.mode.SetSelected("TCP")
			v.host.SetText(active.Host)
			v.port.SetText(strconv.Itoa(active.Port))
		}
	}
	return nil
}
