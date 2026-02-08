package jobs

import "testing"

// TestEventBusSince verifies incremental event reads by sequence.
func TestEventBusSince(t *testing.T) {
	bus := NewEventBus(3)
	bus.Publish(Event{Type: EventTypeStatus, Message: "1"})
	bus.Publish(Event{Type: EventTypeStatus, Message: "2"})
	bus.Publish(Event{Type: EventTypeStatus, Message: "3"})

	events := bus.Since(1)
	if len(events) != 2 {
		t.Fatalf("len = %d, want 2", len(events))
	}
	if events[0].Seq != 2 || events[1].Seq != 3 {
		t.Fatalf("unexpected seqs: %+v", events)
	}
}

// TestEventBusCapsHistory verifies buffer limit trimming behavior.
func TestEventBusCapsHistory(t *testing.T) {
	bus := NewEventBus(2)
	bus.Publish(Event{Message: "1"})
	bus.Publish(Event{Message: "2"})
	bus.Publish(Event{Message: "3"})

	events := bus.Since(0)
	if len(events) != 2 {
		t.Fatalf("len = %d, want 2", len(events))
	}
	if events[0].Message != "2" || events[1].Message != "3" {
		t.Fatalf("unexpected events: %+v", events)
	}
}
