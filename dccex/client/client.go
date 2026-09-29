// Package client owns one connection's reading, ordered writes, and shutdown.
package client

import (
	"context"
	"errors"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"io"
	"sync"
	"time"
)

// Received separates parse errors (Result.Err, nonterminal) from transport errors.
type Received struct {
	Context context.Context
	Result  p.Result
	Err     error
}
type Client struct {
	telemetry            *telemetry.Manager
	connectionContext    context.Context
	finishConnection     func(error)
	connectionAttributes []attribute.KeyValue
	conn                 io.ReadWriteCloser
	events               chan Received
	done                 chan struct{}
	once                 sync.Once
	causeMu              sync.Mutex
	readCause            error
	writeMu              sync.Mutex
}

func New(conn io.ReadWriteCloser) *Client {
	return NewObserved(conn, nil, nil)
}

func NewObserved(conn io.ReadWriteCloser, observer *telemetry.Manager, attrs []attribute.KeyValue) *Client {
	ctx, finish := observer.Start(context.Background(), "connection.lifetime", attrs...)
	c := &Client{conn: conn, events: make(chan Received, 64), done: make(chan struct{}), telemetry: observer, connectionContext: ctx, finishConnection: finish, connectionAttributes: append([]attribute.KeyValue(nil), attrs...)}
	observer.Active(1)
	observer.Event(ctx, "connection.opened", attrs...)
	go c.read()
	return c
}
func (c *Client) Events() <-chan Received { return c.events }
func (c *Client) Close() error {
	return c.closeWithCause(nil)
}

// The first shutdown owns the lifetime result. Closing a failed transport must
// not overwrite its cause with the usually successful result of conn.Close.
func (c *Client) closeWithCause(cause error) error {
	var err error
	c.once.Do(func() {
		close(c.done)
		err = c.conn.Close()
		c.causeMu.Lock()
		cause = errors.Join(cause, c.readCause)
		c.causeMu.Unlock()
		c.telemetry.Active(-1)
		c.telemetry.Event(c.connectionContext, "connection.closed")
		c.finishConnection(errors.Join(cause, err))
	})
	return err
}
func (c *Client) emit(e Received) bool {
	select {
	case c.events <- e:
		return true
	case <-c.done:
		return false
	}
}
func (c *Client) read() {
	defer close(c.events)
	defer c.Close()
	var d p.Framer
	buf := make([]byte, 1024)
	for {
		n, err := c.conn.Read(buf)
		c.telemetry.Bytes(c.connectionContext, "receive", n)
		for _, frame := range d.Feed(buf[:n]) {
			// Unsolicited broadcasts are independent traces, not invented
			// responses to the last command. Sampling changes apply immediately.
			ctx, finish := c.telemetry.Start(context.Background(), "protocol.decode", c.connectionAttributes...)
			r := p.Result{Err: frame.Err}
			if frame.Err == nil {
				r.Event, r.Err = p.Parse(frame.Frame)
			}
			c.telemetry.Traffic(ctx, "receive", frame.Frame)
			kind, outcome := "unknown", "success"
			if r.Err != nil {
				outcome = "error"
				c.telemetry.Event(ctx, "protocol.decode_failed")
			}
			if r.Event != nil {
				kind = telemetry.CommandKind(r.Event.RawFrame())
			}
			c.telemetry.Message(ctx, "receive", kind, outcome)
			finish(r.Err)
			if !c.emit(Received{Result: r, Context: ctx}) {
				return
			}
		}
		if err != nil {
			select {
			case <-c.done:
				return
			default:
			}
			if end := d.End(); end != nil {
				c.telemetry.Message(c.connectionContext, "receive", "unknown", "truncated")
				if !c.emit(Received{Result: p.Result{Err: end}}) {
					return
				}
			}
			c.telemetry.Event(c.connectionContext, "connection.read_failed")
			// Record the failure before publishing it: the session may immediately
			// call Close after receiving the terminal event.
			c.causeMu.Lock()
			c.readCause = err
			c.causeMu.Unlock()
			c.emit(Received{Err: err})
			return
		}
		if n == 0 {
			select {
			case <-c.done:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
}

// Send writes one ASCII command and newline. Failure closes the connection.
func (c *Client) Send(command string) error {
	return c.SendContext(context.Background(), command)
}

func (c *Client) SendContext(ctx context.Context, command string) (sendErr error) {
	attrs := append([]attribute.KeyValue{attribute.String("command", telemetry.CommandKind(command))}, c.connectionAttributes...)
	ctx, finish := c.telemetry.Start(ctx, "transport.write", attrs...)
	defer func() {
		outcome := "success"
		if sendErr != nil {
			outcome = "error"
			c.telemetry.Event(ctx, "command.write_failed")
		}
		c.telemetry.Message(ctx, "send", telemetry.CommandKind(command), outcome)
		finish(sendErr)
	}()
	c.telemetry.Traffic(ctx, "send", command)
	for _, b := range []byte(command) {
		if b > 127 {
			return errors.New("DCC-EX commands must be ASCII")
		}
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	select {
	case <-c.done:
		return io.ErrClosedPipe
	default:
	}
	result := make(chan error, 1)
	go func() {
		data := []byte(command + "\n")
		for len(data) > 0 {
			n, err := c.conn.Write(data)
			if n > 0 && n <= len(data) {
				c.telemetry.Bytes(ctx, "send", n)
			}
			if err != nil {
				result <- err
				return
			}
			if n <= 0 || n > len(data) {
				result <- io.ErrShortWrite
				return
			}
			data = data[n:]
		}
		result <- nil
	}()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case err := <-result:
		if err != nil {
			c.closeWithCause(err)
		}
		return err
	case <-timer.C:
		c.telemetry.Event(ctx, "command.write_timeout")
		err := errors.New("command write timed out")
		c.closeWithCause(err)
		return err
	case <-c.done:
		return io.ErrClosedPipe
	}
}
