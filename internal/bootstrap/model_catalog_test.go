package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"media-transcriber/internal/domain"
)

// TestGetWhisperModelByID verifies known model lookup.
func TestGetWhisperModelByID(t *testing.T) {
	model, found := getWhisperModelByID("base.en")
	if !found {
		t.Fatal("expected base.en model to exist")
	}
	if model.FileName != "ggml-base.en.bin" {
		t.Fatalf("filename = %s, want ggml-base.en.bin", model.FileName)
	}
}

// TestResolveModelDownloadDirectoryForEmptyPath falls back to default local model directory.
func TestResolveModelDownloadDirectoryForEmptyPath(t *testing.T) {
	dir, err := resolveModelDownloadDirectory("")
	if err != nil {
		t.Fatalf("resolve dir: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(dir), "/.media-transcriber/models") {
		t.Fatalf("dir = %s, expected ~/.media-transcriber/models suffix", dir)
	}
}

// TestResolveModelDownloadDirectoryForModelFile uses model file parent directory.
func TestResolveModelDownloadDirectoryForModelFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ggml-small.bin")

	dir, err := resolveModelDownloadDirectory(path)
	if err != nil {
		t.Fatalf("resolve dir: %v", err)
	}
	if dir != root {
		t.Fatalf("dir = %s, want %s", dir, root)
	}
}

// TestResolveModelDownloadDirectoryForExistingDirectory keeps that directory.
func TestResolveModelDownloadDirectoryForExistingDirectory(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("mkdir models dir: %v", err)
	}

	dir, err := resolveModelDownloadDirectory(modelsDir)
	if err != nil {
		t.Fatalf("resolve dir: %v", err)
	}
	if dir != modelsDir {
		t.Fatalf("dir = %s, want %s", dir, modelsDir)
	}
}

// TestResolveModelDownloadDirectoryRejectsExistingNonModelFile rejects invalid file path.
func TestResolveModelDownloadDirectoryRejectsExistingNonModelFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("not model"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := resolveModelDownloadDirectory(file); err == nil {
		t.Fatal("expected error for existing non-model file path")
	}
}

// TestMarkDownloadedModels marks catalog models when file exists in known dirs.
func TestMarkDownloadedModels(t *testing.T) {
	root := t.TempDir()
	modelPath := filepath.Join(root, "ggml-base.en.bin")
	if err := os.WriteFile(modelPath, []byte("stub"), 0o644); err != nil {
		t.Fatalf("write model file: %v", err)
	}

	models := []domain.WhisperModelOption{
		{ID: "base.en", FileName: "ggml-base.en.bin"},
		{ID: "small", FileName: "ggml-small.bin"},
	}
	markDownloadedModels(models, []string{root})

	if !models[0].Downloaded {
		t.Fatal("expected base.en to be marked downloaded")
	}
	if models[0].LocalPath != modelPath {
		t.Fatalf("localPath = %s, want %s", models[0].LocalPath, modelPath)
	}
	if models[1].Downloaded {
		t.Fatal("expected small to remain not downloaded")
	}
}
