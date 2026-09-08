package client_test

import (
	"bufio"
	"errors"
	c "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/client"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func next(t *testing.T, ch <-chan c.Received) c.Received {
	t.Helper()
	select {
	case e, ok := <-ch:
		if !ok {
			t.Fatal("unexpected closed channel")
		}
		return e
	case <-time.After(time.Second):
		t.Fatal("event timed out")
	}
	return c.Received{}
}
func TestClientFramesAndEOF(t *testing.T) {
	local, remote := net.Pipe()
	client := c.New(local)
	defer client.Close()
	go func() {
		remote.Write([]byte("<p"))
		remote.Write([]byte("1><v 29 bad><r 300><unfinished"))
		remote.Close()
	}()
	if _, ok := next(t, client.Events()).Result.Event.(p.TrackPower); !ok {
		t.Fatal("power")
	}
	if !errors.Is(next(t, client.Events()).Result.Err, p.ErrMalformed) {
		t.Fatal("parse error")
	}
	if _, ok := next(t, client.Events()).Result.Event.(p.AddressResult); !ok {
		t.Fatal("reader died on malformed frame")
	}
	if !errors.Is(next(t, client.Events()).Result.Err, p.ErrTruncated) {
		t.Fatal("truncated frame")
	}
	if !errors.Is(next(t, client.Events()).Err, io.EOF) {
		t.Fatal("EOF not terminal")
	}
}
func TestConcurrentWritesRemainWhole(t *testing.T) {
	local, remote := net.Pipe()
	client := c.New(local)
	defer client.Close()
	defer remote.Close()
	got := make(chan string, 20)
	go func() {
		scan := bufio.NewScanner(remote)
		for scan.Scan() {
			got <- scan.Text()
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := client.Send("<t 3 0 1>"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 20; i++ {
		select {
		case text := <-got:
			if text != "<t 3 0 1>" {
				t.Fatal("interleaved command", text)
			}
		case <-time.After(time.Second):
			t.Fatal("missing command")
		}
	}
	if err := client.Send("é"); err == nil {
		t.Fatal("non-ASCII accepted")
	}
}

type idleSerial struct {
	read   int
	closed chan struct{}
	once   sync.Once
}

func (s *idleSerial) Read(b []byte) (int, error) {
	s.read++
	if s.read == 1 {
		return 0, nil
	}
	if s.read == 2 {
		return copy(b, "<p0>"), nil
	}
	<-s.closed
	return 0, io.EOF
}
func (s *idleSerial) Write(b []byte) (int, error) { return len(b), nil }
func (s *idleSerial) Close() error                { s.once.Do(func() { close(s.closed) }); return nil }
func TestSerialTimeoutIsNotEOF(t *testing.T) {
	client := c.New(&idleSerial{closed: make(chan struct{})})
	defer client.Close()
	if _, ok := next(t, client.Events()).Result.Event.(p.TrackPower); !ok {
		t.Fatal("idle read disconnected")
	}
}
func TestCloseUnblocksFullEventQueue(t *testing.T) {
	local, remote := net.Pipe()
	client := c.New(local)
	defer remote.Close()
	go func() { remote.Write([]byte(strings.Repeat("<p0>", 300))) }()
	// Close may happen before or during queue filling; either must terminate.
	client.Close()
	done := make(chan struct{})
	go func() {
		for range client.Events() {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reader leaked")
	}
}
func TestFailedWriteDisconnects(t *testing.T) {
	local, remote := net.Pipe()
	client := c.New(local)
	remote.Close()
	if err := client.Send("<s>"); err == nil {
		t.Fatal("failed write succeeded")
	}
	if err := client.Send("<s>"); err == nil {
		t.Fatal("failed connection reused")
	}
	client.Close()
}
