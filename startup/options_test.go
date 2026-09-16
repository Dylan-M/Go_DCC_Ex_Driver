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
		if o.AutoConnect != (len(args) != 0) {
			t.Fatal("auto-connect must require explicit connection arguments", o)
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

func TestPartialConnectionArgumentsAutoConnect(t *testing.T) {
	for _, args := range [][]string{{"--host", "localhost"}, {"--port=50825"}, {"--host=192.168.4.1"}} {
		var output bytes.Buffer
		o, err := Parse(args, &output)
		if err != nil || !o.AutoConnect {
			t.Fatal(args, o, err)
		}
		if args[0] == "--port=50825" && (o.Host != "192.168.4.1" || o.Port != 50825) {
			t.Fatal("port-only must use default host", o)
		}
		if args[0] == "--host" && (o.Host != "localhost" || o.Port != 2560) {
			t.Fatal("host-only must use default port", o)
		}
	}
}
