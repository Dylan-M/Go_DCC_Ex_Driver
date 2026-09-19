package throttle_test

import (
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"testing"
)

func TestLabelAcceptedBeforeCloseIsSaved(t *testing.T) {
	var saved config.ThrottleSettings
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: func(value config.ThrottleSettings) error { saved = value; return nil }})
	if err := s.SetFunctionLabel(3, 28, "Last label"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	waitClosed(t, s.Done())
	if len(saved.Tabs) != 1 || saved.Tabs[0].Labels[28] != "Last label" {
		t.Fatal(saved)
	}
}
