package transcribe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Request contains input media and execution callbacks for one run.
type Request struct {
	InputPath string
	ModelPath string
	Language  string
	OutputDir string
	OnStage   func(stage string)
	OnLog     func(log CommandLog)
}

// Result contains output artifact paths, transcript text, and command logs.
type Result struct {
	PreprocessedAudioPath string
	TextPath              string
	Transcript            string
	Logs                  []CommandLog
	tempDir               string
}

// Cleanup removes temporary preprocessing artifacts created by Run.
func (r *Result) Cleanup() error {
	if r == nil || r.tempDir == "" {
		return nil
	}

	if err := os.RemoveAll(r.tempDir); err != nil {
		return err
	}
	r.tempDir = ""
	return nil
}

// CommandLog captures one external command invocation result.
type CommandLog struct {
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	ExitCode int      `json:"exitCode"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
}

// PipelineError is a stage-aware error with optional command context.
type PipelineError struct {
	Stage      string     `json:"stage"`
	Message    string     `json:"message"`
	CommandLog CommandLog `json:"commandLog"`
	Err        error      `json:"-"`
}

// Error formats pipeline failures for logs and UI.
func (e *PipelineError) Error() string {
	if e == nil {
		return ""
	}
	if e.CommandLog.Command == "" {
		return fmt.Sprintf("%s: %s", e.Stage, e.Message)
	}

	return fmt.Sprintf(
		"%s: %s (cmd=%s exit=%d)",
		e.Stage,
		e.Message,
		e.CommandLog.Command,
		e.CommandLog.ExitCode,
	)
}

// Unwrap exposes underlying error for errors.Is / errors.As.
func (e *PipelineError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// commandResult is an internal process execution response.
type commandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// commandRunner abstracts process execution for testability.
type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) (commandResult, error)
}

// execRunner executes commands via os/exec.
type execRunner struct{}

// Run executes one command and captures stdout/stderr and exit code.
func (r *execRunner) Run(ctx context.Context, name string, args ...string) (commandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := commandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
	if err != nil {
		result.ExitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
		return result, err
	}

	return result, nil
}

// Pipeline orchestrates ffmpeg preprocessing and whisper transcription.
type Pipeline struct {
	ffmpegPath  string
	whisperPath string
	runner      commandRunner
	mkdirTemp   func(dir, pattern string) (string, error)
	removeAll   func(path string) error
	stat        func(name string) (os.FileInfo, error)
	mkdirAll    func(path string, perm os.FileMode) error
	readDir     func(name string) ([]os.DirEntry, error)
	readFile    func(name string) ([]byte, error)
}

// NewPipeline constructs the production pipeline with OS dependencies.
func NewPipeline() *Pipeline {
	return &Pipeline{
		ffmpegPath:  "ffmpeg",
		whisperPath: "whisper.cpp",
		runner:      &execRunner{},
		mkdirTemp:   os.MkdirTemp,
		removeAll:   os.RemoveAll,
		stat:        os.Stat,
		mkdirAll:    os.MkdirAll,
		readDir:     os.ReadDir,
		readFile:    os.ReadFile,
	}
}

// Run performs preprocessing, transcription, and transcript export.
func (p *Pipeline) Run(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.InputPath) == "" {
		return Result{}, &PipelineError{
			Stage:   "preprocessing",
			Message: "input media path is required",
		}
	}

	if _, err := p.stat(req.InputPath); err != nil {
		return Result{}, &PipelineError{
			Stage:   "preprocessing",
			Message: fmt.Sprintf("cannot access input media: %s", req.InputPath),
			Err:     err,
		}
	}

	modelPath, err := p.resolveModelPath(req.ModelPath)
	if err != nil {
		return Result{}, &PipelineError{
			Stage:   "transcribing",
			Message: err.Error(),
			Err:     err,
		}
	}

	if strings.TrimSpace(req.OutputDir) == "" {
		return Result{}, &PipelineError{
			Stage:   "exporting",
			Message: "output directory is required",
		}
	}
	if err := p.mkdirAll(req.OutputDir, 0o755); err != nil {
		return Result{}, &PipelineError{
			Stage:   "exporting",
			Message: fmt.Sprintf("cannot create output directory: %s", req.OutputDir),
			Err:     err,
		}
	}

	tempDir, err := p.mkdirTemp("", "media-transcriber-*")
	if err != nil {
		return Result{}, &PipelineError{
			Stage:   "preprocessing",
			Message: "failed to create temporary workspace",
			Err:     err,
		}
	}

	outPath := filepath.Join(tempDir, "preprocessed-16k-mono.wav")
	emitStage(req.OnStage, "preprocessing")
	args := buildFFmpegArgs(req.InputPath, outPath)

	cmdResult, runErr := p.runner.Run(ctx, p.ffmpegPath, args...)
	log := CommandLog{
		Command:  p.ffmpegPath,
		Args:     args,
		ExitCode: cmdResult.ExitCode,
		Stdout:   cmdResult.Stdout,
		Stderr:   cmdResult.Stderr,
	}
	emitLog(req.OnLog, log)
	if runErr != nil {
		_ = p.removeAll(tempDir)
		return Result{}, &PipelineError{
			Stage:      "preprocessing",
			Message:    "ffmpeg audio conversion failed",
			CommandLog: log,
			Err:        runErr,
		}
	}

	if _, err := p.stat(outPath); err != nil {
		_ = p.removeAll(tempDir)
		return Result{}, &PipelineError{
			Stage:      "preprocessing",
			Message:    "ffmpeg completed but output file is missing",
			CommandLog: log,
			Err:        err,
		}
	}

	textPath := filepath.Join(req.OutputDir, transcriptFileName(req.InputPath))
	textBase := strings.TrimSuffix(textPath, filepath.Ext(textPath))
	emitStage(req.OnStage, "transcribing")
	whisperArgs := buildWhisperArgs(modelPath, outPath, textBase, req.Language)

	whisperResult, runErr := p.runner.Run(ctx, p.whisperPath, whisperArgs...)
	whisperLog := CommandLog{
		Command:  p.whisperPath,
		Args:     whisperArgs,
		ExitCode: whisperResult.ExitCode,
		Stdout:   whisperResult.Stdout,
		Stderr:   whisperResult.Stderr,
	}
	emitLog(req.OnLog, whisperLog)
	if runErr != nil {
		_ = p.removeAll(tempDir)
		return Result{}, &PipelineError{
			Stage:      "transcribing",
			Message:    "whisper.cpp transcription failed",
			CommandLog: whisperLog,
			Err:        runErr,
		}
	}

	if _, err := p.stat(textPath); err != nil {
		_ = p.removeAll(tempDir)
		return Result{}, &PipelineError{
			Stage:      "exporting",
			Message:    "whisper.cpp completed but transcript .txt file is missing",
			CommandLog: whisperLog,
			Err:        err,
		}
	}

	emitStage(req.OnStage, "exporting")
	content, err := p.readFile(textPath)
	if err != nil {
		_ = p.removeAll(tempDir)
		return Result{}, &PipelineError{
			Stage:      "exporting",
			Message:    fmt.Sprintf("failed to read transcript file: %s", textPath),
			CommandLog: whisperLog,
			Err:        err,
		}
	}

	return Result{
		PreprocessedAudioPath: outPath,
		TextPath:              textPath,
		Transcript:            strings.TrimSpace(string(content)),
		Logs:                  []CommandLog{log, whisperLog},
		tempDir:               tempDir,
	}, nil
}

// emitStage forwards stage updates when callback is configured.
func emitStage(cb func(stage string), stage string) {
	if cb != nil {
		cb(stage)
	}
}

// emitLog forwards command logs when callback is configured.
func emitLog(cb func(log CommandLog), log CommandLog) {
	if cb != nil {
		cb(log)
	}
}

// resolveModelPath returns model file path from file or directory input.
func (p *Pipeline) resolveModelPath(rawPath string) (string, error) {
	modelPath := strings.TrimSpace(rawPath)
	if modelPath == "" {
		return "", fmt.Errorf("model path is required")
	}

	info, err := p.stat(modelPath)
	if err != nil {
		return "", fmt.Errorf("cannot access model path: %s", modelPath)
	}
	if !info.IsDir() {
		return modelPath, nil
	}

	entries, err := p.readDir(modelPath)
	if err != nil {
		return "", fmt.Errorf("cannot read model directory: %s", modelPath)
	}

	modelNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".bin" || ext == ".gguf" {
			modelNames = append(modelNames, entry.Name())
		}
	}
	if len(modelNames) == 0 {
		return "", fmt.Errorf("no .bin or .gguf model files found in: %s", modelPath)
	}

	sort.Strings(modelNames)
	return filepath.Join(modelPath, modelNames[0]), nil
}

// normalizeLanguage maps "auto" and empty language to no CLI override.
func normalizeLanguage(raw string) string {
	lang := strings.TrimSpace(raw)
	if lang == "" || strings.EqualFold(lang, "auto") {
		return ""
	}
	return lang
}

// buildFFmpegArgs builds preprocessing CLI args for mono 16k PCM WAV output.
func buildFFmpegArgs(inputPath, outPath string) []string {
	return []string{
		"-hide_banner",
		"-nostdin",
		"-y",
		"-i", inputPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		outPath,
	}
}

// buildWhisperArgs builds whisper.cpp args for txt transcript export.
func buildWhisperArgs(modelPath, audioPath, textBase, language string) []string {
	args := []string{
		"-m", modelPath,
		"-f", audioPath,
		"-of", textBase,
		"-otxt",
	}

	if lang := normalizeLanguage(language); lang != "" {
		args = append(args, "-l", lang)
	}

	return args
}

// transcriptFileName builds output text filename from input media name.
func transcriptFileName(inputPath string) string {
	base := filepath.Base(inputPath)
	name := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "transcript"
	}
	return name + ".txt"
}

// NewPipelineForTests constructs a pipeline with injectable dependencies.
func NewPipelineForTests(
	ffmpegPath string,
	whisperPath string,
	runner commandRunner,
	mkdirTemp func(dir, pattern string) (string, error),
	removeAll func(path string) error,
	stat func(name string) (os.FileInfo, error),
) *Pipeline {
	return &Pipeline{
		ffmpegPath:  ffmpegPath,
		whisperPath: whisperPath,
		runner:      runner,
		mkdirTemp:   mkdirTemp,
		removeAll:   removeAll,
		stat:        stat,
		mkdirAll:    os.MkdirAll,
		readDir:     os.ReadDir,
		readFile:    os.ReadFile,
	}
}
