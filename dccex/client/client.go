// Package client owns one connection's reading, ordered writes, and shutdown.
package client

import (
	"errors"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"io"
	"sync"
	"time"
)

// Received separates parse errors (Result.Err, nonterminal) from transport errors.
type Received struct {
	Result p.Result
	Err    error
}
type Client struct {
	conn    io.ReadWriteCloser
	events  chan Received
	done    chan struct{}
	once    sync.Once
	writeMu sync.Mutex
}

func New(conn io.ReadWriteCloser) *Client {
	c := &Client{conn: conn, events: make(chan Received, 64), done: make(chan struct{})}
	go c.read()
	return c
}
func (c *Client) Events() <-chan Received { return c.events }
func (c *Client) Close() error {
	var err error
	c.once.Do(func() { close(c.done); err = c.conn.Close() })
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
	var d p.Decoder
	buf := make([]byte, 1024)
	for {
		n, err := c.conn.Read(buf)
		for _, r := range d.Feed(buf[:n]) {
			if !c.emit(Received{Result: r}) {
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
				if !c.emit(Received{Result: p.Result{Err: end}}) {
					return
				}
			}
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
			c.Close()
		}
		return err
	case <-timer.C:
		c.Close()
		return errors.New("command write timed out")
	case <-c.done:
		return io.ErrClosedPipe
	}
}
