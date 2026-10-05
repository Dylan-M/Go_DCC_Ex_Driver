package protocol_test

import (
	"errors"
	"testing"

	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
)

func TestCommandRejectionMustBeUppercaseWithoutArguments(t *testing.T) {
	event, err := p.Parse("< X >")
	if err != nil {
		t.Fatal(err)
	}
	if rejection, ok := event.(p.CommandRejected); !ok || rejection.RawFrame() != "< X >" {
		t.Fatal("rejection envelope lost", event)
	}
	for _, frame := range []string{"<X 1>", "<X reason>"} {
		if event, err := p.Parse(frame); event != nil || !errors.Is(err, p.ErrMalformed) {
			t.Fatal("accepted rejection with arguments", frame, event, err)
		}
	}
}
