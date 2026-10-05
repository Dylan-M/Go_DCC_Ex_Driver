package capabilities_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/capabilities"
)

func begin(t *testing.T, d *capabilities.Discovery, now time.Time) uint64 {
	t.Helper()
	attempt, err := d.Begin(now, time.Second)
	if err != nil || attempt == 0 || d.Snapshot().Status != capabilities.Pending {
		t.Fatalf("begin: attempt=%d, err=%v", attempt, err)
	}
	return attempt
}

func TestDiscoverySuccessfulAndEmptyReplies(t *testing.T) {
	for _, value := range []capabilities.Catalog{catalog(), {SchemaVersion: 1}} {
		var d capabilities.Discovery
		now := time.Unix(100, 0)
		attempt := begin(t, &d, now)
		if consumed, err := d.Accept(attempt, value, now); !consumed || err != nil {
			t.Fatal(consumed, err)
		}
		want := capabilities.Snapshot{Status: capabilities.Available, Catalog: value}
		if got := d.Snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		if d.Reject(attempt, now) || d.Expire(now.Add(time.Hour)) {
			t.Fatal("completed query changed after rejection or timeout")
		}
		if consumed, err := d.Accept(attempt, catalog(), now); consumed || err != nil {
			t.Fatal("duplicate reply accepted")
		}
		begin(t, &d, now)
		if len(d.Snapshot().Catalog.Features) != 0 {
			t.Fatal("retry retained previous capabilities")
		}
	}
}

func TestDiscoverySnapshotsAndInputAreDetached(t *testing.T) {
	var d capabilities.Discovery
	now := time.Unix(100, 0)
	value := catalog()
	attempt := begin(t, &d, now)
	if consumed, err := d.Accept(attempt, value, now); !consumed || err != nil {
		t.Fatal(consumed, err)
	}
	value.Features[0].Versions[0] = 99
	value.Features[0].Details["limit"] = "changed"
	copy := d.Snapshot()
	copy.Catalog.Features[0].Versions[0] = 88
	copy.Catalog.Features[0].Details["limit"] = "changed again"
	if !reflect.DeepEqual(d.Snapshot().Catalog, catalog()) {
		t.Fatal("discovery shares mutable input or snapshot metadata")
	}
}

func TestDiscoveryValidationAndRejection(t *testing.T) {
	var d capabilities.Discovery
	now := time.Unix(100, 0)
	if attempt, err := d.Begin(now, 0); err == nil || attempt != 0 || d.Snapshot().Status != capabilities.NotRequested {
		t.Fatal("invalid timeout changed state")
	}
	attempt := begin(t, &d, now)
	if next, err := d.Begin(now, time.Second); err == nil || next != 0 {
		t.Fatal("concurrent discovery allowed")
	}
	if d.Reject(attempt+1, now) || d.Snapshot().Status != capabilities.Pending {
		t.Fatal("unrelated rejection consumed")
	}
	if consumed, err := d.Accept(attempt+1, catalog(), now); consumed || err != nil {
		t.Fatal("unrelated reply consumed")
	}
	if !d.Reject(attempt, now) || d.Snapshot().Status != capabilities.Unsupported {
		t.Fatal("rejection was not explicit unsupported-query state")
	}
	attempt = begin(t, &d, now)
	if consumed, err := d.Accept(attempt, capabilities.Catalog{}, now); !consumed || err == nil || d.Snapshot().Status != capabilities.InvalidResponse {
		t.Fatal("invalid catalog enabled capabilities")
	}
}

func TestDiscoveryTimeoutAndLateReplies(t *testing.T) {
	for _, operation := range []string{"timer", "reply", "rejection"} {
		t.Run(operation, func(t *testing.T) {
			var d capabilities.Discovery
			now := time.Unix(100, 0)
			attempt := begin(t, &d, now)
			if d.Expire(now.Add(time.Second - time.Nanosecond)) {
				t.Fatal("expired before deadline")
			}
			deadline := now.Add(time.Second)
			switch operation {
			case "timer":
				if !d.Expire(deadline) {
					t.Fatal("deadline did not expire")
				}
			case "reply":
				if consumed, err := d.Accept(attempt, catalog(), deadline); consumed || err != nil {
					t.Fatal("late reply accepted")
				}
			case "rejection":
				if d.Reject(attempt, deadline) {
					t.Fatal("late rejection accepted")
				}
			}
			if d.Snapshot().Status != capabilities.Inconclusive {
				t.Fatal("timeout was treated as unsupported or available")
			}
		})
	}
}

func TestDiscoveryResetInvalidatesPreviousAttempt(t *testing.T) {
	var d capabilities.Discovery
	now := time.Unix(100, 0)
	old := begin(t, &d, now)
	d.Reset()
	if d.Reject(old, now) || d.Expire(now.Add(time.Hour)) || !reflect.DeepEqual(d.Snapshot(), capabilities.Snapshot{}) {
		t.Fatal("reset retained station state")
	}
	current := begin(t, &d, now)
	if current == old || d.Reject(old, now) {
		t.Fatal("old connection rejection completed new attempt")
	}
	if consumed, err := d.Accept(old, catalog(), now); consumed || err != nil {
		t.Fatal("old connection reply completed new attempt")
	}
	if consumed, err := d.Accept(current, catalog(), now); !consumed || err != nil {
		t.Fatal(consumed, err)
	}
	d.Reset()
	if d.Snapshot().Supports(1, "example.consist", 1) {
		t.Fatal("completed station capabilities survived reset")
	}
}
