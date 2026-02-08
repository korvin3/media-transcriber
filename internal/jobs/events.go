package jobs

import (
	"sync"
	"time"

	"media-transcriber/internal/domain"
)

// EventType classifies messages emitted during job execution.
type EventType string

const (
	EventTypeStatus EventType = "status"
	EventTypeLog    EventType = "log"
	EventTypeResult EventType = "result"
	EventTypeError  EventType = "error"
)

// Event is a sequenced payload consumed by UI subscribers.
type Event struct {
	Seq       int64            `json:"seq"`
	Timestamp time.Time        `json:"timestamp"`
	JobID     string           `json:"jobId"`
	Type      EventType        `json:"type"`
	Status    domain.JobStatus `json:"status,omitempty"`
	Message   string           `json:"message,omitempty"`
	Command   string           `json:"command,omitempty"`
	Args      []string         `json:"args,omitempty"`
	ExitCode  int              `json:"exitCode,omitempty"`
	Stdout    string           `json:"stdout,omitempty"`
	Stderr    string           `json:"stderr,omitempty"`
	TextPath  string           `json:"textPath,omitempty"`
}

// EventBus stores recent events and provides incremental reads.
type EventBus struct {
	mu        sync.RWMutex
	nextSeq   int64
	maxEvents int
	events    []Event
}

// NewEventBus creates a bounded in-memory event buffer.
func NewEventBus(maxEvents int) *EventBus {
	if maxEvents <= 0 {
		maxEvents = 500
	}

	return &EventBus{
		maxEvents: maxEvents,
		events:    make([]Event, 0, maxEvents),
	}
}

// Publish appends one event and assigns sequence and timestamp.
func (b *EventBus) Publish(event Event) Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextSeq++
	event.Seq = b.nextSeq
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	b.events = append(b.events, event)
	if len(b.events) > b.maxEvents {
		trim := len(b.events) - b.maxEvents
		b.events = append([]Event(nil), b.events[trim:]...)
	}

	return event
}

// Since returns events with sequence strictly greater than seq.
func (b *EventBus) Since(seq int64) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.events) == 0 {
		return nil
	}

	out := make([]Event, 0, len(b.events))
	for _, event := range b.events {
		if event.Seq > seq {
			out = append(out, event)
		}
	}
	return out
}
