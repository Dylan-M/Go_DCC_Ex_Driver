package startup

import (
	"bytes"
	"errors"
	"flag"
	"testing"
)

func TestParse(t *testing.T) {
	for _, args := range [][]string{nil, {"--host", "localhost", "--port", "5000"}, {"-host=::1", "-port=65535"}} {
		var output bytes.Buffer
		o, err := Parse(args, &output)
		if err != nil {
			t.Fatal(err)
		}
		if len(args) == 0 && (o.Host != "192.168.4.1" || o.Port != 2560) {
			t.Fatal(o)
		}
		if len(args) == 4 && (o.Host != "localhost" || o.Port != 5000) {
			t.Fatal(o)
		}
		if len(args) == 2 && (o.Host != "::1" || o.Port != 65535) {
			t.Fatal(o)
		}
	}
	for _, args := range [][]string{{"--port", "0"}, {"--port", "65536"}, {"--port", "x"}, {"--host", ""}, {"--host", "http://localhost"}, {"--bogus"}, {"extra"}, {"--host"}} {
		var output bytes.Buffer
		if _, err := Parse(args, &output); err == nil {
			t.Fatal("accepted", args)
		}
	}
	var output bytes.Buffer
	if _, err := Parse([]string{"--help"}, &output); !errors.Is(err, flag.ErrHelp) || output.Len() == 0 {
		t.Fatal("help", err)
	}
}
