package capabilities

import (
	"errors"
	"time"
)

type Status uint8

const (
	NotRequested Status = iota
	Pending
	Available
	Unsupported
	Inconclusive
	InvalidResponse
)

type Snapshot struct {
	Status  Status
	Catalog Catalog
}

func (s Snapshot) Supports(schema uint32, id string, version uint32) bool {
	return s.Status == Available && s.Catalog.Supports(schema, id, version)
}

// Discovery is an event-loop-owned state machine, not a transport. The caller
// must isolate its read-only query from unrelated requests because <X> carries
// no request ID. Attempt tokens reject callbacks from previous connections;
// they do not add correlation to the DCC-EX protocol.
type Discovery struct {
	snapshot Snapshot
	attempt  uint64
	deadline time.Time
}

func (d *Discovery) Snapshot() Snapshot {
	s := d.snapshot
	s.Catalog = s.Catalog.Clone()
	return s
}

// Begin records a future query attempt; it does not send any command. The wire
// adapter must start it only for a query it actually sends, and reset on failure.
func (d *Discovery) Begin(now time.Time, timeout time.Duration) (uint64, error) {
	if timeout <= 0 {
		return 0, errors.New("capability discovery timeout must be positive")
	}
	if d.snapshot.Status == Pending {
		return 0, errors.New("capability discovery is already pending")
	}
	d.attempt++
	d.deadline = now.Add(timeout)
	d.snapshot = Snapshot{Status: Pending}
	return d.attempt, nil
}

func (d *Discovery) Accept(attempt uint64, catalog Catalog, now time.Time) (bool, error) {
	if !d.pending(attempt, now) {
		return false, nil
	}
	if err := catalog.Validate(); err != nil {
		d.snapshot = Snapshot{Status: InvalidResponse}
		return true, err
	}
	d.snapshot = Snapshot{Status: Available, Catalog: catalog.Clone()}
	return true, nil
}

// Reject handles uppercase <X> only when attributed to this query by its caller.
// It must not consume unrelated command errors or treat a timeout as rejection.
func (d *Discovery) Reject(attempt uint64, now time.Time) bool {
	if !d.pending(attempt, now) {
		return false
	}
	d.snapshot = Snapshot{Status: Unsupported}
	return true
}

func (d *Discovery) Expire(now time.Time) bool {
	if d.snapshot.Status != Pending || now.Before(d.deadline) {
		return false
	}
	d.snapshot = Snapshot{Status: Inconclusive}
	return true
}

// Reset discards station-specific capabilities, but preserves attempt numbering
// so a late callback from a disconnected station cannot complete the next query.
func (d *Discovery) Reset() {
	d.snapshot = Snapshot{}
	d.deadline = time.Time{}
}

func (d *Discovery) pending(attempt uint64, now time.Time) bool {
	if d.snapshot.Status != Pending || attempt != d.attempt {
		return false
	}
	return !d.Expire(now)
}
