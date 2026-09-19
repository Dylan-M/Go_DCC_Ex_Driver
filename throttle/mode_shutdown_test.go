package throttle_test

import (
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"testing"
)

func TestModePreferencesFlushOnClose(t *testing.T) {
	var saved config.ThrottleSettings
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: func(value config.ThrottleSettings) error { saved = value; return nil }})
	if err := s.SetFunctionToggle(3, 0, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.FlipFunctionToggle(3, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.FlipFunctionToggle(3, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.FlipFunctionToggle(3, -1); err != nil {
		t.Fatal(err)
	}
	s.Close()
	waitClosed(t, s.Done())
	if len(saved.Tabs) != 1 || saved.Tabs[0].Toggle == nil || !saved.Tabs[0].Toggle[0] || saved.Tabs[0].Toggle[3] {
		t.Fatal(saved)
	}
}
