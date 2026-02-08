package config

import (
	"os"
	"path/filepath"

	"media-transcriber/internal/domain"
)

// DefaultSettings returns baseline local configuration for first launch.
func DefaultSettings() domain.Settings {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	return domain.Settings{
		ModelPath: filepath.Join(homeDir, ".media-transcriber", "models"),
		OutputDir: filepath.Join(homeDir, "Documents", "Transcripts"),
		Language:  "auto",
	}
}
