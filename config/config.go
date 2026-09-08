// Package config preserves toggle_funcs JSON. Encode and Decode also work with
// mobile storage; desktop path policy is confined to the filesystem helpers.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

type Settings struct{ Toggle [29]bool }

func Default() Settings { var s Settings; s.Toggle[3] = true; return s }
func Decode(r io.Reader) (Settings, error) {
	var doc struct {
		Toggle json.RawMessage `json:"toggle_funcs"`
	}
	dec := json.NewDecoder(io.LimitReader(r, 1024*1024))
	if err := dec.Decode(&doc); err != nil {
		return Default(), err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Default(), errors.New("unexpected trailing configuration data")
	}
	var values []json.RawMessage
	if len(doc.Toggle) == 0 || string(doc.Toggle) == "null" {
		return Default(), errors.New("toggle_funcs must be an array")
	}
	if err := json.Unmarshal(doc.Toggle, &values); err != nil {
		return Default(), err
	}
	var s Settings
	for _, raw := range values {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil {
			var text string
			if err = json.Unmarshal(raw, &text); err != nil {
				return Default(), err
			}
			n, err = strconv.Atoi(text)
			if err != nil {
				return Default(), err
			}
		}
		if n >= 0 && n < 29 {
			s.Toggle[n] = true
		}
	}
	return s, nil
}
func Encode(w io.Writer, s Settings) error {
	nums := make([]int, 0)
	for n, on := range s.Toggle {
		if on {
			nums = append(nums, n)
		}
	}
	return json.NewEncoder(w).Encode(struct {
		Toggle []int `json:"toggle_funcs"`
	}{nums})
}
func DesktopPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "dccex-throttle.json"), nil
}
func Load(path string) (Settings, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	defer f.Close()
	return Decode(f)
}
func LoadDesktop() (Settings, string, error) {
	path, err := DesktopPath()
	if err != nil {
		return Default(), "", err
	}
	if _, err = os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		s, e := Load(path)
		return s, path, e
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Default(), path, err
	}
	s, err := Load(filepath.Join(home, ".config", "dccex-throttle.json"))
	return s, path, err
}
func Save(path string, s Settings) error {
	if path == "" {
		return errors.New("configuration path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".dccex-*.tmp")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = Encode(f, s); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(temp, path); err != nil {
		return fmt.Errorf("save configuration: %w", err)
	}
	return nil
}
