package throttle

import (
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"strings"
)

func (c *Controller) receivePower(v p.TrackPower) {
	switch v.Track {
	case "MAIN":
		c.state.MainPower = v.State
	case "PROG":
		c.state.ProgPower = v.State
	case "ALL", "JOIN":
		c.state.MainPower, c.state.ProgPower = v.State, v.State
	default:
		if len(v.Track) == 1 && v.Track[0] >= 'A' && v.Track[0] <= 'H' {
			c.trackPowers[v.Track[0]-'A'] = v.State
		}
	}
	c.updateTrackPower()
}

// Track Manager replies are authoritative when their mapping and power are
// known. In particular, a global <p0> can mean no output is ON while one output
// is overloaded; do not let that summary hide the per-output fault.
func (c *Controller) updateTrackPower() {
	group := func(role string, fallback p.PowerState) p.PowerState {
		count, on, off, unknown := 0, 0, 0, false
		for n, mode := range c.trackModes {
			if mode != role && !strings.HasPrefix(mode, role+"_") {
				continue
			}
			count++
			switch c.trackPowers[n] {
			case p.Overload:
				return p.Overload
			case p.On:
				on++
			case p.Off:
				off++
			default:
				unknown = true
			}
		}
		if count == 0 {
			return fallback
		}
		if unknown {
			return ""
		}
		if on == count {
			return p.On
		}
		if off == count {
			return p.Off
		}
		return "MIXED"
	}
	c.state.MainPower = group("MAIN", c.state.MainPower)
	c.state.ProgPower = group("PROG", c.state.ProgPower)
	c.state.Overload = c.state.MainPower == p.Overload || c.state.ProgPower == p.Overload
}
