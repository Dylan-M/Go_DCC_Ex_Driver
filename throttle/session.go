package throttle

import (
	"context"
	"errors"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/client"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/transport"
	"io"
	"reflect"
	"sync"
	"time"
)

type Connection struct {
	Serial bool
	Host   string
	Port   int
	Device string
	Baud   int
}
type Opener func(context.Context, Connection) (io.ReadWriteCloser, error)

func Open(ctx context.Context, o Connection) (io.ReadWriteCloser, error) {
	if o.Serial {
		return transport.Serial(o.Device, o.Baud)
	}
	return transport.TCP(ctx, o.Host, o.Port)
}

// Session serializes UI intents, station replies, and timers. Updates contain
// snapshots, never live controller data. Slow views receive the latest state.
type Session struct {
	queueMu  sync.Mutex // serializes acceptance with Close
	ctx      context.Context
	cancel   context.CancelFunc
	actions  chan sessionAction
	updates  chan State
	done     chan struct{}
	opener   Opener
	saveTabs func(config.ThrottleSettings) error
	current  *client.Client // owned by run
}

type sessionAction struct {
	apply      func(*Controller) error
	preference bool
}

func NewSession(settings config.Settings, opener Opener, tabs ...TabPersistence) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	if opener == nil {
		opener = Open
	}
	s := &Session{ctx: ctx, cancel: cancel, actions: make(chan sessionAction, 128), updates: make(chan State, 1), done: make(chan struct{}), opener: opener}
	c := New(settings.Toggle)
	if len(tabs) > 0 {
		if err := c.restoreTabs(tabs[0].Initial); err != nil {
			c.Log("err", "Saved throttles unavailable: "+err.Error())
		} else {
			s.saveTabs = tabs[0].Save
		}
	}
	go s.run(c)
	return s
}
func (s *Session) Updates() <-chan State { return s.updates }
func (s *Session) Done() <-chan struct{} { return s.done }

// Close rejects new work immediately. Done closes after accepted local
// preferences are saved; queued operating commands are never replayed.
func (s *Session) Close() {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	s.cancel()
}
func (s *Session) Post(fn func(*Controller) error) error {
	return s.enqueue(sessionAction{apply: fn})
}

// RenameCab queues a local preference that survives an immediate Close.
func (s *Session) RenameCab(cab int, name string) error {
	return s.enqueue(sessionAction{preference: true, apply: func(c *Controller) error { return c.RenameCab(cab, name) }})
}

func (s *Session) enqueue(action sessionAction) error {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	select {
	case <-s.ctx.Done():
		return errors.New("application is closing")
	default:
	}
	select {
	case s.actions <- action:
		return nil
	default:
		return errors.New("controller is busy; command was not queued")
	}
}
func (s *Session) Connect(o Connection) error {
	return s.Post(func(c *Controller) error {
		if c.Snapshot().Connected {
			c.Detach("Disconnected")
			s.current = nil
			return nil
		}
		conn, err := s.opener(s.ctx, o)
		if err != nil {
			return err
		}
		if s.ctx.Err() != nil {
			conn.Close()
			return s.ctx.Err()
		}
		s.current = client.New(conn)
		where := o.Host
		if o.Serial {
			where = o.Device
		}
		err = c.Attach(s.current, where)
		if c.state.Connected {
			c.state.ActiveConnection = o
		}
		return err
	})
}
func (s *Session) publish(state State) {
	select {
	case s.updates <- state:
	default:
		select {
		case <-s.updates:
		default:
		}
		select {
		case s.updates <- state:
		default:
		}
	}
}
func (s *Session) run(c *Controller) {
	defer close(s.done)
	defer close(s.updates)
	defer c.Close()
	defer s.flushPreferences(c)
	speed := time.NewTicker(30 * time.Millisecond)
	poll := time.NewTicker(time.Second)
	defer speed.Stop()
	defer poll.Stop()
	previous := c.Snapshot()
	s.publish(previous)
	for {
		if s.ctx.Err() != nil {
			return
		}
		var events <-chan client.Received
		if s.current != nil {
			events = s.current.Events()
		}
		select {
		case <-s.ctx.Done():
			return
		case action := <-s.actions:
			s.applyAction(c, action)
		case now := <-speed.C:
			if s.ctx.Err() == nil {
				c.Tick(now)
			}
		case <-poll.C:
			if s.ctx.Err() == nil {
				c.Poll()
			}
		case e, ok := <-events:
			if !ok {
				c.Detach("Disconnected: connection closed")
				s.current = nil
			} else if e.Err != nil {
				c.Detach("Disconnected: " + e.Err.Error())
				c.Log("err", c.Snapshot().Status)
				s.current = nil
			} else if e.Result.Err != nil {
				c.Log("err", e.Result.Err.Error())
			} else {
				c.Receive(e.Result.Event)
			}
		}
		next := c.Snapshot()
		if !next.Connected {
			s.current = nil
		}
		if !reflect.DeepEqual(next, previous) {
			s.publish(next)
			previous = next
		}
	}
}

func (s *Session) applyAction(c *Controller, action sessionAction) {
	if s.ctx.Err() != nil {
		if !action.preference {
			return
		}
		c.Detach("Disconnected")
	}
	before := c.tabSettings()
	if err := action.apply(c); err != nil {
		c.Log("err", err.Error())
	}
	// Live state is never saved. A failed station query after a valid layout
	// change must not prevent persistence of that local change.
	after := c.tabSettings()
	if s.saveTabs != nil && !reflect.DeepEqual(before, after) {
		if err := s.saveTabs(after); err != nil {
			c.Log("err", "Could not save throttle tabs: "+err.Error())
		}
	}
}

func (s *Session) flushPreferences(c *Controller) {
	s.Close()
	c.Detach("Disconnected")
	for {
		select {
		case action := <-s.actions:
			s.applyAction(c, action)
		default:
			s.publish(c.Snapshot())
			return
		}
	}
}
