package jobs

import (
	"errors"
	"fmt"
	"sync"

	"media-transcriber/internal/domain"
)

// ErrJobAlreadyRunning is returned when starting a second active job.
var ErrJobAlreadyRunning = errors.New("job already running")

// ErrNoRunningJob is returned when cancel is requested for idle state.
var ErrNoRunningJob = errors.New("no running job")

// Manager tracks the single allowed active job and its transitions.
type Manager struct {
	mu      sync.RWMutex
	current domain.Job
}

// NewManager creates a manager in idle state.
func NewManager() *Manager {
	return &Manager{
		current: domain.Job{
			Status: domain.JobStatusIdle,
		},
	}
}

// Start creates a new job and moves it to preprocessing state.
func (m *Manager) Start(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if isRunning(m.current.Status) {
		return ErrJobAlreadyRunning
	}

	m.current = domain.Job{
		ID:     jobID,
		Status: domain.JobStatusPreprocessing,
	}
	return nil
}

// Transition validates and applies state transitions for current job.
func (m *Manager) Transition(status domain.JobStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current.ID == "" && status != domain.JobStatusIdle {
		return fmt.Errorf("cannot transition without an active job")
	}
	if status == m.current.Status {
		return nil
	}
	if !isValidTransition(m.current.Status, status) {
		return fmt.Errorf("invalid transition: %s -> %s", m.current.Status, status)
	}

	m.current.Status = status
	return nil
}

// Current returns a snapshot of the current job.
func (m *Manager) Current() domain.Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Reset clears job metadata and returns manager to idle.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = domain.Job{Status: domain.JobStatusIdle}
}

// IsRunning reports whether the current state is an active stage.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return isRunning(m.current.Status)
}

// Cancel moves an active job to cancelled state.
func (m *Manager) Cancel() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !isRunning(m.current.Status) {
		return ErrNoRunningJob
	}
	m.current.Status = domain.JobStatusCancelled
	return nil
}

// isRunning checks if a status represents active pipeline execution.
func isRunning(status domain.JobStatus) bool {
	switch status {
	case domain.JobStatusPreprocessing, domain.JobStatusTranscribing, domain.JobStatusExporting:
		return true
	default:
		return false
	}
}

// isValidTransition enforces the allowed job state machine edges.
func isValidTransition(from, to domain.JobStatus) bool {
	switch from {
	case domain.JobStatusIdle:
		return to == domain.JobStatusPreprocessing
	case domain.JobStatusPreprocessing:
		return to == domain.JobStatusTranscribing || to == domain.JobStatusFailed || to == domain.JobStatusCancelled
	case domain.JobStatusTranscribing:
		return to == domain.JobStatusExporting || to == domain.JobStatusFailed || to == domain.JobStatusCancelled
	case domain.JobStatusExporting:
		return to == domain.JobStatusDone || to == domain.JobStatusFailed || to == domain.JobStatusCancelled
	case domain.JobStatusDone, domain.JobStatusFailed, domain.JobStatusCancelled:
		return to == domain.JobStatusPreprocessing || to == domain.JobStatusIdle
	default:
		return false
	}
}
