package config

import (
	"os"
	"path/filepath"
	"testing"

	"media-transcriber/internal/domain"
)

// TestDefaultSettings verifies baseline defaults are present.
func TestDefaultSettings(t *testing.T) {
	cfg := DefaultSettings()
	if cfg.Language != "auto" {
		t.Fatalf("language = %q, want auto", cfg.Language)
	}
	if cfg.ModelPath == "" {
		t.Fatal("expected non-empty model path")
	}
	if cfg.OutputDir == "" {
		t.Fatal("expected non-empty output dir")
	}
}

// TestJSONStoreLoadMissingReturnsDefaults checks first-run behavior.
func TestJSONStoreLoadMissingReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "settings.json")
	store := NewJSONStore(path)

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Language != "auto" {
		t.Fatalf("language = %q, want auto", got.Language)
	}
}

// TestJSONStoreSaveAndLoadRoundTrip checks persisted settings fidelity.
func TestJSONStoreSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "settings.json")
	store := NewJSONStore(path)
	want := domain.Settings{
		ModelPath: "/models/base.bin",
		OutputDir: "/out",
		Language:  "en",
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
}

// TestJSONStoreLoadInvalidJSON checks parse error handling.
func TestJSONStoreLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	store := NewJSONStore(path)
	if _, err := store.Load(); err == nil {
		t.Fatal("expected json parse error")
	}
}
