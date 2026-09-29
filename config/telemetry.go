package config

import (
	"errors"
	"math"
	"net/url"
	"strings"
	"unicode/utf8"
)

// TelemetrySettings is stored with the other app preferences. The zero value
// never enables export. Endpoint is an OTLP/HTTP base URL, not a station URL.
type TelemetrySettings struct {
	Version         int     `json:"version"`
	Enabled         bool    `json:"enabled"`
	Endpoint        string  `json:"endpoint"`
	Debug           bool    `json:"debug"`
	SampleRatio     float64 `json:"sample_ratio"`
	IntervalSeconds int     `json:"interval_seconds"`
}

func DefaultTelemetry() TelemetrySettings {
	return TelemetrySettings{Version: 1, SampleRatio: 1, IntervalSeconds: 15}
}

func (s TelemetrySettings) Validate() error {
	if s.Version != 1 {
		return errors.New("unsupported telemetry settings version")
	}
	if math.IsNaN(s.SampleRatio) || math.IsInf(s.SampleRatio, 0) || s.SampleRatio < 0 || s.SampleRatio > 1 {
		return errors.New("trace sampling must be between 0 and 1")
	}
	if s.IntervalSeconds < 5 || s.IntervalSeconds > 300 {
		return errors.New("metric interval must be 5–300 seconds")
	}
	if s.Endpoint == "" && !s.Enabled {
		return nil
	}
	u, err := url.Parse(s.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || !utf8.ValidString(u.Host) || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.TrimSpace(s.Endpoint) != s.Endpoint {
		return errors.New("collector must be an http(s) base URL without credentials, query, or fragment")
	}
	return nil
}
