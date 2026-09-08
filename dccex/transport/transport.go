// Package transport opens byte streams independently of application state.
package transport

import (
	"context"
	"fmt"
	"go.bug.st/serial"
	"net"
	"time"
)

func TCP(ctx context.Context, host string, port int) (net.Conn, error) {
	if host == "" || port < 1 || port > 65535 {
		return nil, fmt.Errorf("host and port 1-65535 required")
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
}
func Serial(name string, baud int) (serial.Port, error) {
	if name == "" || baud <= 0 {
		return nil, fmt.Errorf("serial port and positive baud rate required")
	}
	port, err := serial.Open(name, &serial.Mode{BaudRate: baud, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit})
	if err != nil {
		return nil, err
	}
	if err = port.SetReadTimeout(400 * time.Millisecond); err != nil {
		port.Close()
		return nil, err
	}
	time.Sleep(200 * time.Millisecond)
	if err = port.ResetInputBuffer(); err != nil {
		port.Close()
		return nil, err
	}
	return port, nil
}
func Ports() ([]string, error) { return serial.GetPortsList() }
