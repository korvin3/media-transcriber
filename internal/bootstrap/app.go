package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"media-transcriber/internal/config"
	"media-transcriber/internal/diagnostics"
	"media-transcriber/internal/domain"
	"media-transcriber/internal/jobs"
	"media-transcriber/internal/transcribe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var mediaDialogFilter = []wailsruntime.FileFilter{
	{
		DisplayName: "Media files",
		Pattern:     "*.mp4;*.mov;*.mkv;*.avi;*.mp3;*.wav;*.m4a;*.flac;*.aac;*.ogg;*.webm",
	},
	{
		DisplayName: "All files",
		Pattern:     "*",
	},
}

var modelDialogFilter = []wailsruntime.FileFilter{
	{
		DisplayName: "Whisper models",
		Pattern:     "*.bin;*.gguf",
	},
	{
		DisplayName: "All files",
		Pattern:     "*",
	},
}

// App wires configuration, jobs, pipeline, and UI runtime callbacks.
type App struct {
	Settings    domain.Settings
	Store       config.Store
	Jobs        *jobs.Manager
	Pipeline    pipelineRunner
	Diagnostics domain.DiagnosticReport
	assets      fs.FS
	checker     *diagnostics.Checker

	mu          sync.Mutex
	activeJobID string
	cancel      context.CancelFunc
	events      *jobs.EventBus
	runtimeCtx  context.Context
}

// pipelineRunner isolates the transcription pipeline behind an interface.
type pipelineRunner interface {
	Run(ctx context.Context, req transcribe.Request) (transcribe.Result, error)
}

// New builds the application with persisted settings and startup diagnostics.
func New() (*App, error) {
	return NewWithAssets(nil)
}

// NewWithAssets builds the application and optionally configures embedded frontend assets.
func NewWithAssets(assets fs.FS) (*App, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home: %w", err)
	}
	if err := ensureLocalBinOnPATH(homeDir); err != nil {
		return nil, fmt.Errorf("prepare local tool path: %w", err)
	}

	store := config.NewJSONStore(filepath.Join(homeDir, ".media-transcriber", "settings.json"))
	settings, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	checker := diagnostics.NewChecker()
	report := checker.Run(settings)

	return &App{
		Settings:    settings,
		Store:       store,
		Jobs:        jobs.NewManager(),
		Pipeline:    transcribe.NewPipeline(),
		Diagnostics: report,
		assets:      assets,
		checker:     checker,
		events:      jobs.NewEventBus(1000),
	}, nil
}

// Run starts the Wails desktop application and binds backend methods.
func (a *App) Run() error {
	assetOptions := &assetserver.Options{}
	if a.assets != nil {
		assetOptions.Assets = a.assets
	} else {
		assetOptions.Handler = http.FileServer(http.Dir("./frontend"))
	}

	return wails.Run(&options.App{
		Title:       "Media Transcriber",
		Width:       1180,
		Height:      780,
		AssetServer: assetOptions,
		OnStartup:   a.Startup,
		OnShutdown: func(ctx context.Context) {
			a.mu.Lock()
			defer a.mu.Unlock()
			a.runtimeCtx = nil
		},
		Bind: []interface{}{a},
	})
}

// Startup stores Wails runtime context for push events.
func (a *App) Startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runtimeCtx = ctx
}

// GetDiagnostics returns the latest cached diagnostics report.
func (a *App) GetDiagnostics() domain.DiagnosticReport {
	return a.Diagnostics
}

// GetSettings loads and returns the latest persisted settings.
func (a *App) GetSettings() (domain.Settings, error) {
	settings, err := a.Store.Load()
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}

	a.mu.Lock()
	a.Settings = settings
	a.mu.Unlock()

	return settings, nil
}

// SaveSettings normalizes and persists settings, then refreshes diagnostics.
func (a *App) SaveSettings(settings domain.Settings) (domain.Settings, error) {
	normalized := normalizeSettings(settings)
	if err := a.Store.Save(normalized); err != nil {
		return domain.Settings{}, fmt.Errorf("save settings: %w", err)
	}

	a.mu.Lock()
	a.Settings = normalized
	if a.checker != nil {
		a.Diagnostics = a.checker.Run(normalized)
	}
	a.mu.Unlock()

	return normalized, nil
}

// PickInputFile opens a native file dialog for media selection.
func (a *App) PickInputFile() (string, error) {
	ctx, err := a.runtimeContext()
	if err != nil {
		return "", err
	}

	path, err := wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:   "Select media file",
		Filters: mediaDialogFilter,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(path), nil
}

// PickModelFile opens a native file dialog for whisper model selection.
func (a *App) PickModelFile() (string, error) {
	ctx, err := a.runtimeContext()
	if err != nil {
		return "", err
	}

	path, err := wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:   "Select whisper model",
		Filters: modelDialogFilter,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(path), nil
}

// PickModelDirectory opens a native directory picker for model folders.
func (a *App) PickModelDirectory() (string, error) {
	ctx, err := a.runtimeContext()
	if err != nil {
		return "", err
	}

	path, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "Select model directory",
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(path), nil
}

// PickOutputDirectory opens a native directory picker for transcript exports.
func (a *App) PickOutputDirectory() (string, error) {
	ctx, err := a.runtimeContext()
	if err != nil {
		return "", err
	}

	path, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: "Select output directory",
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(path), nil
}

// OpenOutputFolder opens the given path (or configured output dir) in file manager.
func (a *App) OpenOutputFolder(path string) error {
	target := strings.TrimSpace(path)
	if target == "" {
		a.mu.Lock()
		target = a.Settings.OutputDir
		a.mu.Unlock()
	}
	if target == "" {
		return fmt.Errorf("output path is empty")
	}

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}

	openPath := target
	if !info.IsDir() {
		openPath = filepath.Dir(target)
	}

	return openInFileManager(openPath)
}

// RefreshDiagnostics reloads settings and reruns dependency checks.
func (a *App) RefreshDiagnostics() (domain.DiagnosticReport, error) {
	settings, err := a.Store.Load()
	if err != nil {
		return domain.DiagnosticReport{}, fmt.Errorf("load settings: %w", err)
	}

	a.Settings = settings
	a.Diagnostics = a.checker.Run(settings)
	return a.Diagnostics, nil
}

// StartTranscription creates a job and runs it asynchronously.
func (a *App) StartTranscription(inputPath string) (domain.Job, error) {
	settings, err := a.Store.Load()
	if err != nil {
		return domain.Job{}, fmt.Errorf("load settings: %w", err)
	}

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	if err := a.Jobs.Start(jobID); err != nil {
		return domain.Job{}, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.activeJobID = jobID
	a.cancel = cancel
	a.mu.Unlock()

	a.Settings = settings
	a.publishStatus(jobID, domain.JobStatusPreprocessing, "Job started")

	go a.runTranscriptionJob(ctx, jobID, inputPath, settings)
	return a.Jobs.Current(), nil
}

// CancelTranscription cancels the currently running job, if any.
func (a *App) CancelTranscription() error {
	a.mu.Lock()
	cancel := a.cancel
	activeJobID := a.activeJobID
	a.mu.Unlock()

	if cancel == nil {
		return jobs.ErrNoRunningJob
	}

	cancel()
	if err := a.Jobs.Cancel(); err != nil && !errors.Is(err, jobs.ErrNoRunningJob) {
		return err
	}

	if activeJobID != "" {
		a.publishStatus(activeJobID, domain.JobStatusCancelled, "Cancellation requested")
	}
	return nil
}

// CurrentJob returns current job metadata and status.
func (a *App) CurrentJob() domain.Job {
	return a.Jobs.Current()
}

// JobEvents returns all events with sequence greater than sinceSeq.
func (a *App) JobEvents(sinceSeq int64) []jobs.Event {
	return a.events.Since(sinceSeq)
}

// runTranscriptionJob executes pipeline and maps outcomes to job events.
func (a *App) runTranscriptionJob(ctx context.Context, jobID, inputPath string, settings domain.Settings) {
	req := transcribe.Request{
		InputPath: inputPath,
		ModelPath: settings.ModelPath,
		Language:  settings.Language,
		OutputDir: settings.OutputDir,
		OnStage: func(stage string) {
			status, ok := mapStageToStatus(stage)
			if !ok {
				return
			}
			if err := a.Jobs.Transition(status); err == nil {
				a.publishStatus(jobID, status, "Running "+stage+" stage")
			}
		},
		OnLog: func(log transcribe.CommandLog) {
			a.publishEvent(jobs.Event{
				JobID:    jobID,
				Type:     jobs.EventTypeLog,
				Message:  "Command completed",
				Command:  log.Command,
				Args:     log.Args,
				ExitCode: log.ExitCode,
				Stdout:   log.Stdout,
				Stderr:   log.Stderr,
			})
		},
	}

	result, err := a.Pipeline.Run(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			_ = a.Jobs.Transition(domain.JobStatusCancelled)
			a.publishStatus(jobID, domain.JobStatusCancelled, "Job cancelled")
			a.clearActiveJob(jobID)
			return
		}

		_ = a.Jobs.Transition(domain.JobStatusFailed)
		a.publishStatus(jobID, domain.JobStatusFailed, "Job failed")
		a.publishEvent(jobs.Event{
			JobID:   jobID,
			Type:    jobs.EventTypeError,
			Status:  domain.JobStatusFailed,
			Message: err.Error(),
		})

		var pipelineErr *transcribe.PipelineError
		if errors.As(err, &pipelineErr) && pipelineErr.CommandLog.Command != "" {
			a.publishEvent(jobs.Event{
				JobID:    jobID,
				Type:     jobs.EventTypeLog,
				Message:  "Failed command",
				Command:  pipelineErr.CommandLog.Command,
				Args:     pipelineErr.CommandLog.Args,
				ExitCode: pipelineErr.CommandLog.ExitCode,
				Stdout:   pipelineErr.CommandLog.Stdout,
				Stderr:   pipelineErr.CommandLog.Stderr,
			})
		}

		a.clearActiveJob(jobID)
		return
	}

	if cleanupErr := result.Cleanup(); cleanupErr != nil {
		a.publishEvent(jobs.Event{
			JobID:   jobID,
			Type:    jobs.EventTypeError,
			Message: fmt.Sprintf("cleanup temporary files: %v", cleanupErr),
		})
	}

	if err := a.Jobs.Transition(domain.JobStatusDone); err == nil {
		a.publishStatus(jobID, domain.JobStatusDone, "Job completed")
	}
	a.publishEvent(jobs.Event{
		JobID:    jobID,
		Type:     jobs.EventTypeResult,
		Status:   domain.JobStatusDone,
		Message:  "Transcript exported",
		TextPath: result.TextPath,
	})
	a.clearActiveJob(jobID)
}

// publishStatus sends a normalized status event.
func (a *App) publishStatus(jobID string, status domain.JobStatus, message string) {
	a.publishEvent(jobs.Event{
		JobID:   jobID,
		Type:    jobs.EventTypeStatus,
		Status:  status,
		Message: message,
	})
}

// publishEvent stores event history and emits runtime push notifications.
func (a *App) publishEvent(event jobs.Event) {
	published := a.events.Publish(event)

	a.mu.Lock()
	ctx := a.runtimeCtx
	a.mu.Unlock()
	if ctx != nil {
		wailsruntime.EventsEmit(ctx, "job:event", published)
	}
}

// clearActiveJob clears cancellation handles for completed job IDs.
func (a *App) clearActiveJob(jobID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeJobID == jobID {
		a.activeJobID = ""
		a.cancel = nil
	}
}

// mapStageToStatus maps pipeline stage names to job statuses.
func mapStageToStatus(stage string) (domain.JobStatus, bool) {
	switch stage {
	case "preprocessing":
		return domain.JobStatusPreprocessing, true
	case "transcribing":
		return domain.JobStatusTranscribing, true
	case "exporting":
		return domain.JobStatusExporting, true
	default:
		return "", false
	}
}

// runtimeContext returns current Wails runtime context for dialog APIs.
func (a *App) runtimeContext() (context.Context, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runtimeCtx == nil {
		return nil, fmt.Errorf("runtime context is not initialized")
	}
	return a.runtimeCtx, nil
}

// normalizeSettings trims user inputs and applies default language when empty.
func normalizeSettings(settings domain.Settings) domain.Settings {
	settings.ModelPath = strings.TrimSpace(settings.ModelPath)
	settings.OutputDir = strings.TrimSpace(settings.OutputDir)
	settings.Language = strings.TrimSpace(settings.Language)
	if settings.Language == "" {
		settings.Language = "auto"
	}
	return settings
}

// openInFileManager launches the platform file explorer for the provided path.
func openInFileManager(path string) error {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", filepath.Clean(path))
	default:
		cmd = exec.Command("xdg-open", path)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch file manager: %w", err)
	}
	return nil
}
