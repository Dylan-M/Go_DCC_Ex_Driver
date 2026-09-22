// Package startup parses desktop startup options without opening a GUI or DB.
package startup

import (
	"errors"
	"flag"
	"io"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
)

type Options struct {
	Host          string
	Port          int
	AutoConnect   bool
	PowerThrottle bool
}

func Parse(args []string, output io.Writer) (Options, error) {
	o := Options{}
	flags := flag.NewFlagSet("dccex-driver", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&o.Host, "host", stations.DefaultHost, "TCP hostname or IP address (attempts connection on launch)")
	flags.IntVar(&o.Port, "port", stations.DefaultPort, "TCP port, 1-65535 (attempts connection on launch)")
	flags.BoolVar(&o.PowerThrottle, "power-throttle", false, "enable desktop track power, programming, and raw commands (unavailable on Android)")
	if err := flags.Parse(args); err != nil {
		return o, err
	}
	if flags.NArg() != 0 {
		return o, errors.New("unexpected positional arguments")
	}
	var err error
	o.Host, err = stations.NormalizeHost(o.Host)
	if err != nil {
		return o, err
	}
	if o.Port < 1 || o.Port > 65535 {
		return o, errors.New("port must be 1-65535")
	}
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "host" || f.Name == "port" {
			o.AutoConnect = true
		}
	})
	return o, nil
}
