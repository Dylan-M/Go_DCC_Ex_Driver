package config

import (
	"math"
	"testing"
)

func TestTelemetrySettingsValidation(t *testing.T) {
	s := DefaultTelemetry()
	if s.Enabled || s.Debug || s.Validate() != nil {
		t.Fatal("defaults must be disabled and valid", s)
	}
	for _, endpoint := range []string{"", "ftp://host", "http://", "http://user:password@host", "http://host?token=secret", "http://host#fragment", " http://host", "http://%bad"} {
		s.Enabled, s.Endpoint = true, endpoint
		if s.Validate() == nil {
			t.Errorf("accepted endpoint %q", endpoint)
		}
	}
	for _, endpoint := range []string{"http://localhost:4318", "https://collector.example/otel", "http://[::1]:4318"} {
		s.Endpoint = endpoint
		if err := s.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, ratio := range []float64{-1, 1.01, math.NaN(), math.Inf(1)} {
		s.SampleRatio = ratio
		if s.Validate() == nil {
			t.Fatal("accepted invalid sampling", ratio)
		}
	}
	s.SampleRatio = 0
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, seconds := range []int{0, 4, 301} {
		s.IntervalSeconds = seconds
		if s.Validate() == nil {
			t.Fatal("accepted invalid interval")
		}
	}
	s = DefaultTelemetry()
	s.Version = 2
	if s.Validate() == nil {
		t.Fatal("accepted future schema")
	}
}
