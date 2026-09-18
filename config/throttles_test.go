package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	c "github.com/Dylan-M/Go_DCC_Ex_Driver/config"
)

func TestThrottleSettingsRoundTrip(t *testing.T) {
	want := c.ThrottleSettings{Version: 1, Tabs: []c.ThrottleTab{{Address: 42}, {Address: 3}, {Address: 10293}}, Selected: 3}
	var b bytes.Buffer
	if err := c.EncodeThrottles(&b, want); err != nil {
		t.Fatal(err)
	}
	got, err := c.DecodeThrottles(&b)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	path := filepath.Join(t.TempDir(), "private", "throttles.json")
	got, err = c.LoadThrottles(path)
	if err != nil || !reflect.DeepEqual(got, c.DefaultThrottles()) {
		t.Fatal(got, err)
	}
	for _, settings := range []c.ThrottleSettings{want, c.DefaultThrottles(), want} {
		if err := c.SaveThrottles(path, settings); err != nil {
			t.Fatal(err)
		}
		got, err = c.LoadThrottles(path)
		if err != nil || !reflect.DeepEqual(got, settings) {
			t.Fatal(got, err)
		}
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatal("temporary file leaked", files, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := want
	bad.Selected = 999
	if c.SaveThrottles(path, bad) == nil {
		t.Fatal("invalid settings saved")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("invalid save changed existing file", err)
	}
}

func TestInvalidThrottleSettings(t *testing.T) {
	for _, input := range []string{
		``, `{}`, `null`,
		`{"version":2,"tabs":[{"address":3}],"selected":3}`,
		`{"version":1,"tabs":[],"selected":3}`,
		`{"version":1,"tabs":[{"address":3},{"address":3}],"selected":3}`,
		`{"version":1,"tabs":[{"address":0}],"selected":0}`,
		`{"version":1,"tabs":[{"address":10294}],"selected":10294}`,
		`{"version":1,"tabs":[{"address":3}],"selected":7}`,
		`{"version":1,"tabs":[{"address":3,"speed":50}],"selected":3}`,
		`{"version":1,"tabs":[{"address":3}],"selected":3} {}`,
		`{"version":1,"tabs":[{"address":3}],"selected":3} junk`,
		`{"locos":[{"address":3,"name":"Python profile"}]}`,
		`{"version":1,"tabs":[{"address":3}],"selected":3}` + strings.Repeat(" ", 1024*1024),
	} {
		got, err := c.DecodeThrottles(strings.NewReader(input))
		if err == nil || !reflect.DeepEqual(got, c.DefaultThrottles()) {
			t.Fatalf("%s: %+v, %v", input, got, err)
		}
	}
}
