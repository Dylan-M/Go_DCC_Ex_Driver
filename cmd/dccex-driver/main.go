package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	fyneui "github.com/Dylan-M/Go_DCC_Ex_Driver/ui/fyne"
)

func main() {
	settings, path, loadErr := config.LoadDesktop()
	a := app.NewWithID("com.github.Dylan-M.Go_DCC_Ex_Driver")
	window := a.NewWindow("DCC-EX Native Throttle")
	session := throttle.NewSession(settings, nil, func(s config.Settings) error { return config.Save(path, s) })
	view := fyneui.New(window, session)
	if loadErr != nil {
		session.Post(func(c *throttle.Controller) error { c.Log("err", "Configuration: "+loadErr.Error()); return nil })
	}
	go func() {
		for state := range session.Updates() {
			s := state
			fyne.DoAndWait(func() { view.Render(s) })
		}
	}()
	closing := false
	window.SetCloseIntercept(func() {
		if closing {
			return
		}
		closing = true
		session.Close()
		go func() { <-session.Done(); fyne.Do(func() { window.SetCloseIntercept(nil); window.Close() }) }()
	})
	window.ShowAndRun()
	session.Close()
	<-session.Done()
}
