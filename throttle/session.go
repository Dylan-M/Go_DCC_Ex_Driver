package throttle

import (
	"context"
	"errors"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/client"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/transport"
	"io"
	"reflect"
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
	ctx     context.Context
	cancel  context.CancelFunc
	actions chan func(*Controller) error
	updates chan State
	done    chan struct{}
	opener  Opener
	save    func(config.Settings) error
	current *client.Client // owned by run
}

func NewSession(settings config.Settings, opener Opener, save func(config.Settings) error) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	if opener == nil {
		opener = Open
	}
	s := &Session{ctx: ctx, cancel: cancel, actions: make(chan func(*Controller) error, 128), updates: make(chan State, 1), done: make(chan struct{}), opener: opener, save: save}
	go s.run(New(settings.Toggle))
	return s
}
func (s *Session) Updates() <-chan State { return s.updates }
func (s *Session) Done() <-chan struct{} { return s.done }
func (s *Session) Close()                { s.cancel() }
func (s *Session) Post(fn func(*Controller) error) error {
	select {
	case <-s.ctx.Done():
		return errors.New("application is closing")
	default:
	}
	select {
	case s.actions <- fn:
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
		return c.Attach(s.current, where)
	})
}
func (s *Session) SetToggle(n int, on bool) error {
	return s.Post(func(c *Controller) error {
		if err := c.SetToggle(n, on); err != nil {
			return err
		}
		if s.save != nil {
			return s.save(config.Settings{Toggle: c.Snapshot().Toggle})
		}
		return nil
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
	speed := time.NewTicker(30 * time.Millisecond)
	poll := time.NewTicker(time.Second)
	defer speed.Stop()
	defer poll.Stop()
	previous := c.Snapshot()
	s.publish(previous)
	for {
		var events <-chan client.Received
		if s.current != nil {
			events = s.current.Events()
		}
		select {
		case <-s.ctx.Done():
			return
		case fn := <-s.actions:
			if err := fn(c); err != nil {
				c.Log("err", err.Error())
			}
		case now := <-speed.C:
			c.Tick(now)
		case <-poll.C:
			c.Poll()
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
