// Package stations stores named connection profiles independently of any UI.
package stations

import (
	"errors"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"
)

const DefaultHost = "192.168.4.1"
const DefaultPort = 2560

type Profile struct {
	Name   string `json:"name"`
	Mode   string `json:"mode"`
	Host   string `json:"host,omitempty"`
	Port   int    `json:"port,omitempty"`
	Device string `json:"device,omitempty"`
	Baud   int    `json:"baud,omitempty"`
}

type Repository interface {
	List() ([]Profile, error)
	Save(Profile, bool) error
	Delete(string) error
}

func NormalizeHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		if addr, err := netip.ParseAddr(host[1 : len(host)-1]); err == nil && addr.Is6() {
			return addr.String(), nil
		}
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String(), nil
	}
	if host == "" || len(host) > 253 || !utf8.ValidString(host) {
		return "", errors.New("host must be a hostname or IP address")
	}
	for _, ch := range host {
		if !(unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '.' || ch == '-' || ch == '_') {
			return "", errors.New("host must not include a URL, port, path, or whitespace")
		}
	}
	return host, nil
}

func Normalize(p Profile) (Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || !utf8.ValidString(p.Name) || utf8.RuneCountInString(p.Name) > 80 {
		return p, errors.New("station name must contain 1-80 characters")
	}
	for _, ch := range p.Name {
		if unicode.IsControl(ch) {
			return p, errors.New("station name contains control characters")
		}
	}
	switch p.Mode {
	case "TCP":
		var err error
		p.Host, err = NormalizeHost(p.Host)
		if err != nil {
			return p, err
		}
		if p.Port < 1 || p.Port > 65535 {
			return p, errors.New("TCP port must be 1-65535")
		}
		p.Device, p.Baud = "", 0
	case "Serial":
		p.Device = strings.TrimSpace(p.Device)
		if p.Device == "" || len(p.Device) > 1024 || !utf8.ValidString(p.Device) || p.Baud < 1 || p.Baud > 4_000_000 {
			return p, errors.New("serial device and baud rate 1-4000000 are required")
		}
		for _, ch := range p.Device {
			if unicode.IsControl(ch) {
				return p, errors.New("serial device contains control characters")
			}
		}
		p.Host, p.Port = "", 0
	default:
		return p, errors.New("connection mode must be TCP or Serial")
	}
	return p, nil
}
