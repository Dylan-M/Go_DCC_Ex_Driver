package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ThrottleSettings contains preferences only. Live speed, direction, functions,
// track power and connection state must always come from the command station.
// This versioned record is independent of Python's configuration format.
type ThrottleSettings struct {
	Version  int           `json:"version"`
	Tabs     []ThrottleTab `json:"tabs"`
	Selected int           `json:"selected"`
}

type ThrottleTab struct {
	Address int        `json:"address"`
	Name    string     `json:"name,omitempty"`
	Labels  [29]string `json:"labels,omitempty"`
}

func DefaultThrottles() ThrottleSettings {
	return ThrottleSettings{Version: 1, Tabs: []ThrottleTab{{Address: 3}}, Selected: 3}
}

func (s ThrottleSettings) Validate() error {
	if s.Version != 1 {
		return fmt.Errorf("unsupported throttle settings version %d", s.Version)
	}
	if len(s.Tabs) == 0 {
		return errors.New("keep at least one saved throttle")
	}
	seen := make(map[int]bool, len(s.Tabs))
	for _, tab := range s.Tabs {
		if err := ValidateDisplayName(tab.Name); err != nil {
			return err
		}
		for _, label := range tab.Labels {
			if err := ValidateDisplayName(label); err != nil {
				return err
			}
		}
		if tab.Address < 1 || tab.Address > 10293 || seen[tab.Address] {
			return fmt.Errorf("invalid or duplicate saved locomotive address %d", tab.Address)
		}
		seen[tab.Address] = true
	}
	if !seen[s.Selected] {
		return errors.New("selected throttle is not in the saved tabs")
	}
	return nil
}

func DecodeThrottles(r io.Reader) (ThrottleSettings, error) {
	data, err := io.ReadAll(io.LimitReader(r, 1024*1024+1))
	if err != nil {
		return DefaultThrottles(), err
	}
	if len(data) > 1024*1024 {
		return DefaultThrottles(), errors.New("throttle settings exceed 1 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	// A future version must not silently lose fields when opened by this build.
	dec.DisallowUnknownFields()
	var s ThrottleSettings
	if err := dec.Decode(&s); err != nil {
		return DefaultThrottles(), err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return DefaultThrottles(), errors.New("unexpected trailing throttle settings data")
	}
	if err := s.Validate(); err != nil {
		return DefaultThrottles(), err
	}
	return s, nil
}

func EncodeThrottles(w io.Writer, s ThrottleSettings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(s)
}

func LoadThrottles(path string) (ThrottleSettings, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultThrottles(), nil
	}
	if err != nil {
		return DefaultThrottles(), err
	}
	defer f.Close()
	return DecodeThrottles(f)
}

func SaveThrottles(path string, s ThrottleSettings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if path == "" {
		return errors.New("throttle settings path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".throttles-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = EncodeThrottles(f, s); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
