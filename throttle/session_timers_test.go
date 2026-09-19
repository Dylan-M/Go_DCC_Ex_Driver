package throttle_test

import (
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"testing"
	"time"
)

type timerSender struct{ commands chan string }

func (s timerSender) Send(command string) error { s.commands <- command; return nil }
func (s timerSender) Close() error              { return nil }

func TestSessionTimersOperateBeforeShutdown(t *testing.T) {
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles()})
	station := timerSender{commands: make(chan string, 32)}
	if err := s.Post(func(c *th.Controller) error {
		if err := c.Attach(station, "test"); err != nil {
			return err
		}
		return c.MoveSpeed(27)
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	sawSpeed, sawPoll := false, false
	for !sawSpeed || !sawPoll {
		select {
		case command := <-station.commands:
			sawSpeed = sawSpeed || command == "<t 3 27 1>"
			sawPoll = sawPoll || command == "<c>"
		case <-deadline.C:
			t.Fatal("session timers did not run", sawSpeed, sawPoll)
		}
	}
	s.Close()
	waitClosed(t, s.Done())
}
