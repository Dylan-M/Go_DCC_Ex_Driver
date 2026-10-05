package throttle

import (
	"reflect"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/capabilities"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
)

type discoverySender struct{ commands []string }

func (s *discoverySender) Send(command string) error {
	s.commands = append(s.commands, command)
	return nil
}
func (*discoverySender) Close() error { return nil }

func TestDiscoveryScaffoldDoesNotChangeHandshakeOrConsumeUnrelatedErrors(t *testing.T) {
	c := New([29]bool{})
	s := &discoverySender{}
	if err := c.Attach(s, "fixture"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.commands, []string{"<=>", "<s>", "<t 3>"}) || c.Snapshot().Discovery.Status != capabilities.NotRequested {
		t.Fatal("scaffold sent a guessed capability command", s.commands)
	}
	event, err := p.Parse("<X>")
	if err != nil {
		t.Fatal(err)
	}
	c.Receive(event)
	if !c.Snapshot().Connected || c.Snapshot().Discovery.Status != capabilities.NotRequested {
		t.Fatal("unrelated command rejection disconnected or disabled discovery")
	}
	if last := c.Snapshot().Logs[len(c.Snapshot().Logs)-1]; last.Kind != "rx" || last.Text != "<< <X>" {
		t.Fatal("command rejection lost from diagnostic console", last)
	}
}

func TestControllerDiscoverySnapshotsAndReconnect(t *testing.T) {
	c := New([29]bool{})
	now := time.Unix(100, 0)
	attempt, err := c.discovery.Begin(now, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	catalog := capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{{ID: "example", Versions: []uint32{1}, Details: map[string]string{"limit": "8"}}}}
	if consumed, err := c.discovery.Accept(attempt, catalog, now); !consumed || err != nil {
		t.Fatal(consumed, err)
	}
	snapshot := c.Snapshot()
	snapshot.Discovery.Catalog.Features[0].Versions[0] = 99
	snapshot.Discovery.Catalog.Features[0].Details["limit"] = "changed"
	if !reflect.DeepEqual(c.Snapshot().Discovery.Catalog, catalog) {
		t.Fatal("controller snapshot shares capability metadata")
	}
	c.Detach("test disconnect")
	if !reflect.DeepEqual(c.Snapshot().Discovery, capabilities.Snapshot{}) {
		t.Fatal("capabilities survived disconnect")
	}
	if err := c.Attach(&discoverySender{}, "different station"); err != nil {
		t.Fatal(err)
	}
	if c.Snapshot().Discovery.Supports(1, "example", 1) {
		t.Fatal("new station inherited previous capabilities")
	}
}
