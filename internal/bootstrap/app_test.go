package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"media-transcriber/internal/domain"
	"media-transcriber/internal/jobs"
	"media-transcriber/internal/transcribe"
)

// fakeStore returns deterministic settings for App tests.
type fakeStore struct {
	settings domain.Settings
}

// Load returns preconfigured settings.
func (s *fakeStore) Load() (domain.Settings, error) {
	return s.settings, nil
}

// Save is a no-op for tests.
func (s *fakeStore) Save(domain.Settings) error {
	return nil
}

// fakePipeline allows injecting custom run behavior per test.
type fakePipeline struct {
	run func(ctx context.Context, req transcribe.Request) (transcribe.Result, error)
}

// Run delegates to injected function.
func (p *fakePipeline) Run(ctx context.Context, req transcribe.Request) (transcribe.Result, error) {
	if p.run == nil {
		return transcribe.Result{}, nil
	}
	return p.run(ctx, req)
}

// TestStartTranscriptionEnforcesSingleRunningJob checks single-job guard.
func TestStartTranscriptionEnforcesSingleRunningJob(t *testing.T) {
	store := &fakeStore{
		settings: domain.Settings{
			ModelPath: "/tmp/model.bin",
			OutputDir: t.TempDir(),
			Language:  "auto",
		},
	}

	app := &App{
		Store: store,
		Jobs:  jobs.NewManager(),
		Pipeline: &fakePipeline{run: func(ctx context.Context, req transcribe.Request) (transcribe.Result, error) {
			<-ctx.Done()
			return transcribe.Result{}, ctx.Err()
		}},
		events: jobs.NewEventBus(100),
	}

	if _, err := app.StartTranscription("/tmp/input.mp4"); err != nil {
		t.Fatalf("start first job: %v", err)
	}
	if _, err := app.StartTranscription("/tmp/input-2.mp4"); !errors.Is(err, jobs.ErrJobAlreadyRunning) {
		t.Fatalf("second start error = %v, want %v", err, jobs.ErrJobAlreadyRunning)
	}

	if err := app.CancelTranscription(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	waitForStatus(t, app, domain.JobStatusCancelled)
}

// TestStartTranscriptionPublishesProgressAndResultEvents checks event flow.
func TestStartTranscriptionPublishesProgressAndResultEvents(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "out")
	store := &fakeStore{
		settings: domain.Settings{
			ModelPath: "/tmp/model.bin",
			OutputDir: outputDir,
			Language:  "en",
		},
	}

	app := &App{
		Store: store,
		Jobs:  jobs.NewManager(),
		Pipeline: &fakePipeline{run: func(ctx context.Context, req transcribe.Request) (transcribe.Result, error) {
			if req.OnStage != nil {
				req.OnStage("preprocessing")
				req.OnStage("transcribing")
				req.OnStage("exporting")
			}
			if req.OnLog != nil {
				req.OnLog(transcribe.CommandLog{Command: "ffmpeg", ExitCode: 0})
				req.OnLog(transcribe.CommandLog{Command: "whisper.cpp", ExitCode: 0})
			}
			outPath := filepath.Join(outputDir, "clip.txt")
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return transcribe.Result{}, err
			}
			if err := os.WriteFile(outPath, []byte("hello"), 0o644); err != nil {
				return transcribe.Result{}, err
			}
			return transcribe.Result{
				TextPath:   outPath,
				Transcript: "hello",
			}, nil
		}},
		events: jobs.NewEventBus(100),
	}

	if _, err := app.StartTranscription(filepath.Join(root, "clip.mp4")); err != nil {
		t.Fatalf("start job: %v", err)
	}

	waitForStatus(t, app, domain.JobStatusDone)
	events := app.JobEvents(0)
	if len(events) == 0 {
		t.Fatal("expected events")
	}

	assertEventTypeExists(t, events, jobs.EventTypeStatus)
	assertEventTypeExists(t, events, jobs.EventTypeLog)
	assertEventTypeExists(t, events, jobs.EventTypeResult)
}

// TestStartTranscriptionPublishesFailureEvents checks error path emissions.
func TestStartTranscriptionPublishesFailureEvents(t *testing.T) {
	root := t.TempDir()
	store := &fakeStore{
		settings: domain.Settings{
			ModelPath: "/tmp/model.bin",
			OutputDir: filepath.Join(root, "out"),
			Language:  "en",
		},
	}

	app := &App{
		Store: store,
		Jobs:  jobs.NewManager(),
		Pipeline: &fakePipeline{run: func(ctx context.Context, req transcribe.Request) (transcribe.Result, error) {
			return transcribe.Result{}, &transcribe.PipelineError{
				Stage:   "transcribing",
				Message: "whisper failed",
				CommandLog: transcribe.CommandLog{
					Command:  "whisper.cpp",
					Args:     []string{"-m", "/tmp/model.bin"},
					ExitCode: 1,
					Stderr:   "bad model",
				},
				Err: errors.New("exit status 1"),
			}
		}},
		events: jobs.NewEventBus(100),
	}

	if _, err := app.StartTranscription(filepath.Join(root, "clip.mp4")); err != nil {
		t.Fatalf("start job: %v", err)
	}

	waitForStatus(t, app, domain.JobStatusFailed)
	events := app.JobEvents(0)
	if len(events) == 0 {
		t.Fatal("expected events")
	}

	assertEventTypeExists(t, events, jobs.EventTypeStatus)
	assertEventTypeExists(t, events, jobs.EventTypeError)
	assertEventTypeExists(t, events, jobs.EventTypeLog)
}

// waitForStatus polls until job reaches desired status or times out.
func waitForStatus(t *testing.T, app *App, want domain.JobStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if app.CurrentJob().Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status = %s, want %s", app.CurrentJob().Status, want)
}

// assertEventTypeExists verifies at least one event of given type exists.
func assertEventTypeExists(t *testing.T, events []jobs.Event, want jobs.EventType) {
	t.Helper()
	for _, event := range events {
		if event.Type == want {
			return
		}
	}
	t.Fatalf("event type %s not found", want)
}
