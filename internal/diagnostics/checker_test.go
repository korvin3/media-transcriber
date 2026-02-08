package diagnostics

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"media-transcriber/internal/domain"
)

// TestCheckerRunAllPass validates happy-path diagnostics report.
func TestCheckerRunAllPass(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir models: %v", err)
	}
	modelFile := filepath.Join(modelDir, "ggml-base.bin")
	if err := os.WriteFile(modelFile, []byte("stub"), 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}

	outputDir := filepath.Join(root, "output")
	checker := NewCheckerForTests(
		func(name string) (string, error) { return "/usr/local/bin/" + name, nil },
		os.Stat,
		os.ReadDir,
		os.MkdirAll,
		os.CreateTemp,
		os.Remove,
	)

	report := checker.Run(domain.Settings{
		ModelPath: modelDir,
		OutputDir: outputDir,
		Language:  "auto",
	})

	if report.HasFailures {
		t.Fatalf("expected no failures, got %+v", report.Items)
	}
}

// TestCheckerRunMissingToolsAndPaths validates failure reporting.
func TestCheckerRunMissingToolsAndPaths(t *testing.T) {
	checker := NewCheckerForTests(
		func(string) (string, error) { return "", errors.New("not found") },
		os.Stat,
		os.ReadDir,
		os.MkdirAll,
		os.CreateTemp,
		os.Remove,
	)

	report := checker.Run(domain.Settings{
		ModelPath: "/path/that/does/not/exist",
		OutputDir: "",
	})

	if !report.HasFailures {
		t.Fatal("expected failures")
	}

	assertStatusByID(t, report, "tool_ffmpeg", domain.DiagnosticStatusFail)
	assertStatusByID(t, report, "tool_ffprobe", domain.DiagnosticStatusFail)
	assertStatusByID(t, report, "tool_whisper.cpp", domain.DiagnosticStatusFail)
	assertStatusByID(t, report, "model_path", domain.DiagnosticStatusFail)
	assertStatusByID(t, report, "output_dir", domain.DiagnosticStatusFail)
}

// TestCheckerRunModelDirectoryWithoutModelFilesFails validates model check.
func TestCheckerRunModelDirectoryWithoutModelFilesFails(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir models: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "README.txt"), []byte("no model"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	checker := NewCheckerForTests(
		func(name string) (string, error) { return "/usr/local/bin/" + name, nil },
		os.Stat,
		os.ReadDir,
		os.MkdirAll,
		os.CreateTemp,
		os.Remove,
	)
	report := checker.Run(domain.Settings{
		ModelPath: modelDir,
		OutputDir: filepath.Join(root, "output"),
	})

	assertStatusByID(t, report, "model_path", domain.DiagnosticStatusFail)
}

// assertStatusByID checks status for one diagnostic item by ID.
func assertStatusByID(t *testing.T, report domain.DiagnosticReport, id string, want domain.DiagnosticStatus) {
	t.Helper()
	for _, item := range report.Items {
		if item.ID == id {
			if item.Status != want {
				t.Fatalf("item %s: got %s, want %s", id, item.Status, want)
			}
			return
		}
	}
	t.Fatalf("diagnostic item not found: %s", id)
}
