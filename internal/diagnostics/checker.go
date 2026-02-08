package diagnostics

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"media-transcriber/internal/domain"
)

// Checker validates external tools and required filesystem paths.
type Checker struct {
	lookPath   func(string) (string, error)
	stat       func(string) (os.FileInfo, error)
	readDir    func(string) ([]os.DirEntry, error)
	mkdirAll   func(string, os.FileMode) error
	createTemp func(string, string) (*os.File, error)
	remove     func(string) error
}

// NewChecker builds a checker using real OS dependencies.
func NewChecker() *Checker {
	return &Checker{
		lookPath:   exec.LookPath,
		stat:       os.Stat,
		readDir:    os.ReadDir,
		mkdirAll:   os.MkdirAll,
		createTemp: os.CreateTemp,
		remove:     os.Remove,
	}
}

// Run executes all startup checks and returns a combined report.
func (c *Checker) Run(settings domain.Settings) domain.DiagnosticReport {
	items := []domain.DiagnosticItem{
		c.checkTool("ffmpeg"),
		c.checkTool("ffprobe"),
		c.checkTool("whisper.cpp"),
		c.checkModelPath(settings.ModelPath),
		c.checkOutputDir(settings.OutputDir),
	}

	hasFailures := false
	for _, item := range items {
		if item.Status == domain.DiagnosticStatusFail {
			hasFailures = true
			break
		}
	}

	return domain.DiagnosticReport{
		GeneratedAt: time.Now().UTC(),
		HasFailures: hasFailures,
		Items:       items,
	}
}

// checkTool verifies a required CLI executable is on PATH.
func (c *Checker) checkTool(name string) domain.DiagnosticItem {
	path, err := c.lookPath(name)
	if err != nil {
		return domain.DiagnosticItem{
			ID:      "tool_" + name,
			Name:    name,
			Status:  domain.DiagnosticStatusFail,
			Message: fmt.Sprintf("Tool not found in PATH: %s", name),
			Hint:    "Install it and ensure the binary is available on PATH before starting a transcription job.",
		}
	}

	return domain.DiagnosticItem{
		ID:      "tool_" + name,
		Name:    name,
		Status:  domain.DiagnosticStatusPass,
		Message: fmt.Sprintf("Found at %s", path),
	}
}

// checkModelPath validates configured model file or model directory.
func (c *Checker) checkModelPath(modelPath string) domain.DiagnosticItem {
	item := domain.DiagnosticItem{
		ID:   "model_path",
		Name: "Model path",
	}

	if strings.TrimSpace(modelPath) == "" {
		item.Status = domain.DiagnosticStatusFail
		item.Message = "Model path is empty."
		item.Hint = "Set a valid model file path or a directory containing whisper models."
		return item
	}

	info, err := c.stat(modelPath)
	if err != nil {
		item.Status = domain.DiagnosticStatusFail
		if errors.Is(err, os.ErrNotExist) {
			item.Message = fmt.Sprintf("Model path does not exist: %s", modelPath)
		} else {
			item.Message = fmt.Sprintf("Cannot access model path: %s", modelPath)
		}
		item.Hint = "Download a whisper.cpp model and configure the path in settings."
		return item
	}

	if !info.IsDir() {
		item.Status = domain.DiagnosticStatusPass
		item.Message = fmt.Sprintf("Model file found: %s", modelPath)
		return item
	}

	entries, err := c.readDir(modelPath)
	if err != nil {
		item.Status = domain.DiagnosticStatusFail
		item.Message = fmt.Sprintf("Cannot read model directory: %s", modelPath)
		item.Hint = "Check permissions for the model directory."
		return item
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".bin" || ext == ".gguf" {
			item.Status = domain.DiagnosticStatusPass
			item.Message = fmt.Sprintf("Model directory is valid: %s", modelPath)
			return item
		}
	}

	item.Status = domain.DiagnosticStatusFail
	item.Message = fmt.Sprintf("No model files found in directory: %s", modelPath)
	item.Hint = "Place a .bin or .gguf model file in this directory or point to a model file directly."
	return item
}

// checkOutputDir validates output directory existence and write access.
func (c *Checker) checkOutputDir(outputDir string) domain.DiagnosticItem {
	item := domain.DiagnosticItem{
		ID:   "output_dir",
		Name: "Output directory",
	}

	if strings.TrimSpace(outputDir) == "" {
		item.Status = domain.DiagnosticStatusFail
		item.Message = "Output directory is empty."
		item.Hint = "Set an output directory where transcript files can be written."
		return item
	}

	if err := c.mkdirAll(outputDir, 0o755); err != nil {
		item.Status = domain.DiagnosticStatusFail
		item.Message = fmt.Sprintf("Cannot create output directory: %s", outputDir)
		item.Hint = "Choose a writable location or adjust filesystem permissions."
		return item
	}

	tmpFile, err := c.createTemp(outputDir, ".write-check-*")
	if err != nil {
		item.Status = domain.DiagnosticStatusFail
		item.Message = fmt.Sprintf("Output directory is not writable: %s", outputDir)
		item.Hint = "Choose a writable directory for transcript export."
		return item
	}

	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	_ = c.remove(tmpPath)

	item.Status = domain.DiagnosticStatusPass
	item.Message = fmt.Sprintf("Writable directory: %s", outputDir)
	return item
}

// NewCheckerForTests creates checker with injectable dependencies.
func NewCheckerForTests(
	lookPath func(string) (string, error),
	stat func(string) (os.FileInfo, error),
	readDir func(string) ([]os.DirEntry, error),
	mkdirAll func(string, os.FileMode) error,
	createTemp func(string, string) (*os.File, error),
	remove func(string) error,
) *Checker {
	return &Checker{
		lookPath:   lookPath,
		stat:       stat,
		readDir:    readDir,
		mkdirAll:   mkdirAll,
		createTemp: createTemp,
		remove:     remove,
	}
}

// IsNotExist reports whether error represents file-not-found.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
