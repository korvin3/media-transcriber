package domain

import "time"

// DiagnosticStatus indicates whether a single startup check passed.
type DiagnosticStatus string

const (
	DiagnosticStatusPass DiagnosticStatus = "pass"
	DiagnosticStatusFail DiagnosticStatus = "fail"
)

// DiagnosticItem is one startup check result with optional hint.
type DiagnosticItem struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Status  DiagnosticStatus `json:"status"`
	Message string           `json:"message"`
	Hint    string           `json:"hint,omitempty"`
}

// DiagnosticReport aggregates startup checks for UI and API responses.
type DiagnosticReport struct {
	GeneratedAt time.Time        `json:"generatedAt"`
	HasFailures bool             `json:"hasFailures"`
	Items       []DiagnosticItem `json:"items"`
}
