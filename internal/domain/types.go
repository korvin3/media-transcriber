package domain

// JobStatus tracks each pipeline stage for a single transcription job.
type JobStatus string

const (
	JobStatusIdle          JobStatus = "idle"
	JobStatusPreprocessing JobStatus = "preprocessing"
	JobStatusTranscribing  JobStatus = "transcribing"
	JobStatusExporting     JobStatus = "exporting"
	JobStatusDone          JobStatus = "done"
	JobStatusFailed        JobStatus = "failed"
	JobStatusCancelled     JobStatus = "cancelled"
)

// Settings contains user-selectable runtime configuration.
type Settings struct {
	ModelPath string `json:"modelPath"`
	OutputDir string `json:"outputDir"`
	Language  string `json:"language"`
}

// Job stores the current job identity and lifecycle status.
type Job struct {
	ID     string    `json:"id"`
	Status JobStatus `json:"status"`
}
