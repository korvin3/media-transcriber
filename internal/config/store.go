package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"media-transcriber/internal/domain"
)

// Store defines persistence operations for app settings.
type Store interface {
	Load() (domain.Settings, error)
	Save(domain.Settings) error
}

// JSONStore persists settings in a single JSON file on disk.
type JSONStore struct {
	path string
}

// NewJSONStore creates a JSON-backed settings store.
func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

// Load reads settings from disk or returns defaults when missing.
func (s *JSONStore) Load() (domain.Settings, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultSettings(), nil
		}

		return domain.Settings{}, err
	}

	var cfg domain.Settings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return domain.Settings{}, err
	}

	return cfg, nil
}

// Save writes settings as indented JSON and creates parent directories.
func (s *JSONStore) Save(cfg domain.Settings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}
