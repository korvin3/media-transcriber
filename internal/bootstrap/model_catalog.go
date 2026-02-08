package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"media-transcriber/internal/domain"
)

var whisperModelCatalog = []domain.WhisperModelOption{
	{
		ID:          "tiny.en",
		Name:        "Tiny (English)",
		FileName:    "ggml-tiny.en.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en.bin",
		SizeLabel:   "~75 MB",
		Description: "Fastest, English-only model.",
	},
	{
		ID:          "tiny",
		Name:        "Tiny (Multilingual)",
		FileName:    "ggml-tiny.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
		SizeLabel:   "~75 MB",
		Description: "Fastest multilingual model.",
	},
	{
		ID:          "base.en",
		Name:        "Base (English)",
		FileName:    "ggml-base.en.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin",
		SizeLabel:   "~142 MB",
		Description: "Balanced speed/quality, English-only.",
	},
	{
		ID:          "base",
		Name:        "Base (Multilingual)",
		FileName:    "ggml-base.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
		SizeLabel:   "~142 MB",
		Description: "Balanced speed/quality, multilingual.",
	},
	{
		ID:          "small.en",
		Name:        "Small (English)",
		FileName:    "ggml-small.en.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin",
		SizeLabel:   "~466 MB",
		Description: "Higher quality, English-only.",
	},
	{
		ID:          "small",
		Name:        "Small (Multilingual)",
		FileName:    "ggml-small.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
		SizeLabel:   "~466 MB",
		Description: "Higher quality multilingual model.",
	},
	{
		ID:          "medium.en",
		Name:        "Medium (English)",
		FileName:    "ggml-medium.en.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.en.bin",
		SizeLabel:   "~1.5 GB",
		Description: "High quality, English-only.",
	},
	{
		ID:          "medium",
		Name:        "Medium (Multilingual)",
		FileName:    "ggml-medium.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
		SizeLabel:   "~1.5 GB",
		Description: "High quality multilingual model.",
	},
	{
		ID:          "large-v2",
		Name:        "Large v2",
		FileName:    "ggml-large-v2.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v2.bin",
		SizeLabel:   "~2.9 GB",
		Description: "Very high quality multilingual model.",
	},
	{
		ID:          "large-v3",
		Name:        "Large v3",
		FileName:    "ggml-large-v3.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin",
		SizeLabel:   "~2.9 GB",
		Description: "Latest large multilingual model.",
	},
	{
		ID:          "large-v3-turbo",
		Name:        "Large v3 Turbo",
		FileName:    "ggml-large-v3-turbo.bin",
		URL:         "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
		SizeLabel:   "~1.6 GB",
		Description: "Faster large-v3 variant.",
	},
}

// GetWhisperModels returns built-in whisper.cpp model presets for one-click downloads.
func (a *App) GetWhisperModels() []domain.WhisperModelOption {
	models := make([]domain.WhisperModelOption, len(whisperModelCatalog))
	copy(models, whisperModelCatalog)

	settings, settingsErr := a.loadSettingsForModelCatalog()
	modelDirs := resolveKnownModelDirs(settings, settingsErr == nil)
	markDownloadedModels(models, modelDirs)
	return models
}

// DownloadWhisperModel downloads selected whisper.cpp model and updates settings.ModelPath.
func (a *App) DownloadWhisperModel(modelID string) (domain.Settings, error) {
	id := strings.TrimSpace(modelID)
	if id == "" {
		return domain.Settings{}, fmt.Errorf("model id is required")
	}

	model, found := getWhisperModelByID(id)
	if !found {
		return domain.Settings{}, fmt.Errorf("unknown model id: %s", id)
	}

	if a.Store == nil {
		return domain.Settings{}, fmt.Errorf("settings store is not configured")
	}

	settings, err := a.Store.Load()
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}
	settings = normalizeSettings(settings)

	downloadDir, err := resolveModelDownloadDirectory(settings.ModelPath)
	if err != nil {
		return domain.Settings{}, err
	}

	targetPath := filepath.Join(downloadDir, model.FileName)
	if err := downloadURLToFile(targetPath, model.URL, modelDownloadTimeout); err != nil {
		return domain.Settings{}, fmt.Errorf("download model %s: %w", model.Name, err)
	}

	settings.ModelPath = targetPath
	if err := a.Store.Save(settings); err != nil {
		return domain.Settings{}, fmt.Errorf("save settings: %w", err)
	}

	a.refreshDiagnosticsFromSettings(settings)
	return settings, nil
}

func getWhisperModelByID(id string) (domain.WhisperModelOption, bool) {
	for _, model := range whisperModelCatalog {
		if model.ID == id {
			return model, true
		}
	}
	return domain.WhisperModelOption{}, false
}

func resolveModelDownloadDirectory(modelPath string) (string, error) {
	trimmed := strings.TrimSpace(modelPath)
	if trimmed == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return localModelsDir(homeDir), nil
	}

	info, err := os.Stat(trimmed)
	if err == nil {
		if info.IsDir() {
			return trimmed, nil
		}
		ext := strings.ToLower(filepath.Ext(trimmed))
		if ext == ".bin" || ext == ".gguf" {
			return filepath.Dir(trimmed), nil
		}
		return "", fmt.Errorf("model path points to non-model file: %s", trimmed)
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check model path: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(trimmed))
	if ext == ".bin" || ext == ".gguf" {
		return filepath.Dir(trimmed), nil
	}
	return trimmed, nil
}

func (a *App) loadSettingsForModelCatalog() (domain.Settings, error) {
	if a.Store == nil {
		return domain.Settings{}, fmt.Errorf("settings store is not configured")
	}
	settings, err := a.Store.Load()
	if err != nil {
		return domain.Settings{}, err
	}
	return normalizeSettings(settings), nil
}

func resolveKnownModelDirs(settings domain.Settings, hasSettings bool) []string {
	seen := map[string]struct{}{}
	add := func(path string) {
		p := strings.TrimSpace(path)
		if p == "" {
			return
		}
		clean := filepath.Clean(p)
		if clean == "." {
			return
		}
		seen[clean] = struct{}{}
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		add(localModelsDir(homeDir))
	}

	if hasSettings {
		modelPath := strings.TrimSpace(settings.ModelPath)
		if modelPath != "" {
			info, statErr := os.Stat(modelPath)
			if statErr == nil {
				if info.IsDir() {
					add(modelPath)
				} else {
					add(filepath.Dir(modelPath))
				}
			} else if errors.Is(statErr, os.ErrNotExist) {
				ext := strings.ToLower(filepath.Ext(modelPath))
				if ext == ".bin" || ext == ".gguf" {
					add(filepath.Dir(modelPath))
				} else {
					add(modelPath)
				}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for dir := range seen {
		result = append(result, dir)
	}
	return result
}

func markDownloadedModels(models []domain.WhisperModelOption, modelDirs []string) {
	for i := range models {
		for _, dir := range modelDirs {
			candidate := filepath.Join(dir, models[i].FileName)
			info, err := os.Stat(candidate)
			if err != nil || info.IsDir() {
				continue
			}
			models[i].Downloaded = true
			models[i].LocalPath = candidate
			break
		}
	}
}
