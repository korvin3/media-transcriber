package jobs

import (
	"testing"

	"media-transcriber/internal/domain"
)

// TestManagerLifecycle verifies normal progression to done state.
func TestManagerLifecycle(t *testing.T) {
	m := NewManager()
	if m.IsRunning() {
		t.Fatal("new manager should be idle")
	}

	if err := m.Start("job-1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("expected running after start")
	}

	for _, status := range []domain.JobStatus{
		domain.JobStatusTranscribing,
		domain.JobStatusExporting,
		domain.JobStatusDone,
	} {
		if err := m.Transition(status); err != nil {
			t.Fatalf("transition to %s: %v", status, err)
		}
	}

	current := m.Current()
	if current.Status != domain.JobStatusDone {
		t.Fatalf("current status = %s, want done", current.Status)
	}
}

// TestManagerRejectsInvalidTransition checks state machine constraints.
func TestManagerRejectsInvalidTransition(t *testing.T) {
	m := NewManager()
	if err := m.Start("job-1"); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := m.Transition(domain.JobStatusDone); err == nil {
		t.Fatal("expected invalid transition error")
	}
}

// TestManagerCancel verifies cancel behavior and repeated cancel handling.
func TestManagerCancel(t *testing.T) {
	m := NewManager()
	if err := m.Start("job-1"); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := m.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if m.Current().Status != domain.JobStatusCancelled {
		t.Fatalf("status = %s, want cancelled", m.Current().Status)
	}

	if err := m.Cancel(); err != ErrNoRunningJob {
		t.Fatalf("second cancel error = %v, want %v", err, ErrNoRunningJob)
	}
}
