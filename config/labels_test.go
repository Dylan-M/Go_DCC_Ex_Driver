package config

import (
	"bytes"
	"testing"
)

func TestFunctionLabelValidation(t *testing.T) {
	s := DefaultThrottles()
	s.Tabs[0].Labels[28] = "bad\nlabel"
	var encoded bytes.Buffer
	if err := EncodeThrottles(&encoded, s); err == nil {
		t.Fatal("invalid label was encoded")
	}
	s.Tabs[0].Labels[28] = "Whistle"
	if err := EncodeThrottles(&encoded, s); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeThrottles(&encoded)
	if err != nil || got.Tabs[0].Labels[28] != "Whistle" {
		t.Fatal(got, err)
	}
}
