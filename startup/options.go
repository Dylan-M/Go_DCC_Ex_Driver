// Package startup parses desktop startup options without opening a GUI or DB.
package startup

import (
	"errors"
	"flag"
	"io"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
)

type Options struct {
	Host string
	Port int
}

func Parse(args []string, output io.Writer) (Options, error) {
	o := Options{}
	flags := flag.NewFlagSet("dccex-driver", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&o.Host, "host", stations.DefaultHost, "TCP hostname or IP address (prefills UI; does not connect)")
	flags.IntVar(&o.Port, "port", stations.DefaultPort, "TCP port, 1-65535")
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
	return o, nil
}
