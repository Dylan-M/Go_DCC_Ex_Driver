package throttle

import (
	"context"
	"errors"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/client"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/transport"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	"go.opentelemetry.io/otel/attribute"
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
	telemetry *telemetry.Manager
	queueMu   sync.Mutex // serializes acceptance with Close
	ctx       context.Context
	cancel    context.CancelFunc
	actions   chan sessionAction
	updates   chan State
	done      chan struct{}
	opener    Opener
	saveTabs  func(config.ThrottleSettings) error
	current   *client.Client // owned by run
}

type sessionAction struct {
	ctx        context.Context
	finish     func(error)
	apply      func(*Controller) error
	preference bool
}

func NewSession(settings config.Settings, opener Opener, tabs ...TabPersistence) *Session {
	return NewObservedSession(settings, opener, nil, tabs...)
}

func NewObservedSession(settings config.Settings, opener Opener, observer *telemetry.Manager, tabs ...TabPersistence) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	if opener == nil {
		opener = Open
	}
	s := &Session{ctx: ctx, cancel: cancel, actions: make(chan sessionAction, 128), updates: make(chan State, 1), done: make(chan struct{}), opener: opener}
	c := New(settings.Toggle)
	s.telemetry = observer
	c.telemetry = observer
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
		s.telemetry.Event(context.Background(), "action.rejected_closing")
		return errors.New("application is closing")
	default:
	}
	name := "controller.action"
	if action.preference {
		name = "controller.preference"
	}
	action.ctx, action.finish = s.telemetry.Start(context.Background(), name)
	select {
	case s.actions <- action:
		return nil
	default:
		err := errors.New("controller is busy; command was not queued")
		s.telemetry.Event(action.ctx, "action.queue_full")
		action.finish(err)
		return err
	}
}
func (s *Session) Connect(o Connection) error {
	return s.Post(func(c *Controller) error {
		if c.Snapshot().Connected {
			c.Detach("Disconnected")
			s.current = nil
			return nil
		}
		attrs := []attribute.KeyValue{attribute.String("server.address", o.Host), attribute.Int("server.port", o.Port), attribute.String("transport", "tcp")}
		if o.Serial {
			attrs = []attribute.KeyValue{attribute.String("device", o.Device), attribute.Int("baud", o.Baud), attribute.String("transport", "serial")}
		}
		ctx, finish := s.telemetry.Start(c.operationContext, "connection.open", attrs...)
		s.telemetry.Event(ctx, "connection.attempt", attrs...)
		conn, err := s.opener(s.ctx, o)
		if err != nil {
			s.telemetry.Event(ctx, "connection.open_failed", attrs...)
			finish(err)
			return err
		}
		if s.ctx.Err() != nil {
			conn.Close()
			finish(s.ctx.Err())
			return s.ctx.Err()
		}
		finish(nil)
		s.current = client.NewObserved(conn, s.telemetry, attrs)
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
	s.telemetry.Event(context.Background(), "session.started")
	defer close(s.done)
	defer s.telemetry.Event(context.Background(), "session.stopped")
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
				if err := c.Tick(now); err != nil {
					s.telemetry.Event(context.Background(), "throttle.tick_failed")
				}
			}
		case <-poll.C:
			if s.ctx.Err() == nil {
				if err := c.Poll(); err != nil {
					s.telemetry.Event(context.Background(), "station.poll_failed")
				}
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
				previousContext := c.operationContext
				c.operationContext = e.Context
				c.Receive(e.Result.Event)
				c.operationContext = previousContext
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
	var actionErr error
	if action.finish != nil {
		defer func() { action.finish(actionErr) }()
	}
	oldContext := c.operationContext
	if action.ctx != nil {
		c.operationContext = action.ctx
	}
	defer func() { c.operationContext = oldContext }()
	if s.ctx.Err() != nil {
		if !action.preference {
			actionErr = s.ctx.Err()
			s.telemetry.Event(c.operationContext, "action.cancelled")
			return
		}
		c.Detach("Disconnected")
	}
	before := c.tabSettings()
	if err := action.apply(c); err != nil {
		actionErr = err
		s.telemetry.Event(c.operationContext, "action.failed")
		c.Log("err", err.Error())
	}
	// Live state is never saved. A failed station query after a valid layout
	// change must not prevent persistence of that local change.
	after := c.tabSettings()
	if s.saveTabs != nil && !reflect.DeepEqual(before, after) {
		if err := s.saveTabs(after); err != nil {
			actionErr = err
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
