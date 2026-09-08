package config_test

import (
	"bytes"
	c "github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyJSONAndAtomicSave(t *testing.T) {
	s, err := c.Decode(strings.NewReader("{\"toggle_funcs\":[3,0,28,99,-1,\"2\",3]}"))
	if err != nil {
		t.Fatal(err)
	}
	for n := range s.Toggle {
		if s.Toggle[n] != (n == 0 || n == 2 || n == 3 || n == 28) {
			t.Fatal(n, s)
		}
	}
	var b bytes.Buffer
	if err = c.Encode(&b, s); err != nil {
		t.Fatal(err)
	}
	if b.String() != "{\"toggle_funcs\":[0,2,3,28]}\n" {
		t.Fatal(b.String())
	}
	path := filepath.Join(t.TempDir(), "sub", "dccex-throttle.json")
	if err = c.Save(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := c.Load(path)
	if err != nil || !reflect.DeepEqual(s, got) {
		t.Fatal(got, err)
	}
	if err = c.Save(path, c.Default()); err != nil {
		t.Fatal("overwrite", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatal("temporary file leaked")
	}
}
func TestMalformedFallsBack(t *testing.T) {
	for _, input := range []string{"", "{}", "{\"toggle_funcs\":null}", "{\"toggle_funcs\":2}", "{\"toggle_funcs\":[\"x\"]}", "{\"toggle_funcs\":[]}junk"} {
		got, err := c.Decode(strings.NewReader(input))
		if err == nil || got != c.Default() {
			t.Fatalf("%q %+v %v", input, got, err)
		}
	}
	s, err := c.Decode(strings.NewReader("{\"toggle_funcs\":[]}"))
	if err != nil || s.Toggle[3] {
		t.Fatal("empty choices not preserved")
	}
	got, err := c.Load(filepath.Join(t.TempDir(), "missing"))
	if err != nil || got != c.Default() {
		t.Fatal("missing defaults")
	}
}
