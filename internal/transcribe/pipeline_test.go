package transcribe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner simulates command execution order and outcomes.
type fakeRunner struct {
	run func(ctx context.Context, name string, args ...string) (commandResult, error)
}

// Run delegates to injected behavior.
func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (commandResult, error) {
	if f.run == nil {
		return commandResult{}, nil
	}
	return f.run(ctx, name, args...)
}

// TestPipelineRunSuccessAutoLanguage checks full happy path with auto lang.
func TestPipelineRunSuccessAutoLanguage(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "meeting.mp4")
	modelPath := filepath.Join(root, "ggml-base.bin")
	outputDir := filepath.Join(root, "output")
	mustWriteFile(t, inputPath, "media")
	mustWriteFile(t, modelPath, "model")

	call := 0
	var whisperArgs []string
	runner := &fakeRunner{
		run: func(ctx context.Context, name string, args ...string) (commandResult, error) {
			call++
			switch call {
			case 1:
				if name != "ffmpeg-custom" {
					t.Fatalf("command 1 name = %q, want ffmpeg-custom", name)
				}
				outPath := args[len(args)-1]
				mustWriteFile(t, outPath, "wav")
				return commandResult{Stdout: "ffmpeg ok", ExitCode: 0}, nil
			case 2:
				if name != "whisper-custom" {
					t.Fatalf("command 2 name = %q, want whisper-custom", name)
				}
				whisperArgs = append([]string{}, args...)
				base := argValue(args, "-of")
				mustWriteFile(t, base+".txt", "hello world")
				return commandResult{Stdout: "whisper ok", ExitCode: 0}, nil
			default:
				t.Fatalf("unexpected command call: %d", call)
				return commandResult{}, nil
			}
		},
	}

	pipeline := NewPipelineForTests("ffmpeg-custom", "whisper-custom", runner, os.MkdirTemp, os.RemoveAll, os.Stat)
	result, err := pipeline.Run(context.Background(), Request{
		InputPath: inputPath,
		ModelPath: modelPath,
		Language:  "auto",
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if call != 2 {
		t.Fatalf("command calls = %d, want 2", call)
	}
	if len(result.Logs) != 2 {
		t.Fatalf("logs count = %d, want 2", len(result.Logs))
	}
	if result.TextPath != filepath.Join(outputDir, "meeting.txt") {
		t.Fatalf("text path = %q", result.TextPath)
	}
	if result.Transcript != "hello world" {
		t.Fatalf("transcript = %q", result.Transcript)
	}
	if hasArg(whisperArgs, "-l") {
		t.Fatalf("auto language should not pass -l, args=%v", whisperArgs)
	}
	if _, err := os.Stat(result.TextPath); err != nil {
		t.Fatalf("transcript file missing: %v", err)
	}

	if err := result.Cleanup(); err != nil {
		t.Fatalf("cleanup error: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(result.PreprocessedAudioPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp dir cleanup, stat err = %v", err)
	}
}

// TestPipelineRunFFmpegFailureReturnsPreprocessingError checks conversion error path.
func TestPipelineRunFFmpegFailureReturnsPreprocessingError(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "clip.mp4")
	modelPath := filepath.Join(root, "model.bin")
	outputDir := filepath.Join(root, "out")
	mustWriteFile(t, inputPath, "media")
	mustWriteFile(t, modelPath, "model")

	var cleaned string
	runner := &fakeRunner{
		run: func(ctx context.Context, name string, args ...string) (commandResult, error) {
			return commandResult{
				Stderr:   "ffmpeg failed",
				ExitCode: 1,
			}, errors.New("exit status 1")
		},
	}

	pipeline := NewPipelineForTests(
		"ffmpeg",
		"whisper.cpp",
		runner,
		os.MkdirTemp,
		func(path string) error {
			cleaned = path
			return os.RemoveAll(path)
		},
		os.Stat,
	)

	_, err := pipeline.Run(context.Background(), Request{
		InputPath: inputPath,
		ModelPath: modelPath,
		Language:  "auto",
		OutputDir: outputDir,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var pErr *PipelineError
	if !errors.As(err, &pErr) {
		t.Fatalf("error type = %T, want *PipelineError", err)
	}
	if pErr.Stage != "preprocessing" {
		t.Fatalf("stage = %s, want preprocessing", pErr.Stage)
	}
	if pErr.CommandLog.Command != "ffmpeg" {
		t.Fatalf("command = %q, want ffmpeg", pErr.CommandLog.Command)
	}
	if pErr.CommandLog.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1", pErr.CommandLog.ExitCode)
	}
	if strings.TrimSpace(cleaned) == "" {
		t.Fatal("expected temporary directory cleanup")
	}
}

// TestPipelineRunFixedLanguageAndModelDirectory checks model discovery.
func TestPipelineRunFixedLanguageAndModelDirectory(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "clip.mov")
	modelDir := filepath.Join(root, "models")
	outputDir := filepath.Join(root, "out")
	mustWriteFile(t, inputPath, "media")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir models: %v", err)
	}
	// lexical sort should pick this first.
	mustWriteFile(t, filepath.Join(modelDir, "a-small.gguf"), "model")
	mustWriteFile(t, filepath.Join(modelDir, "z-large.bin"), "model")

	var usedModel string
	var usedLanguage string
	runner := &fakeRunner{
		run: func(ctx context.Context, name string, args ...string) (commandResult, error) {
			if name == "ffmpeg" {
				mustWriteFile(t, args[len(args)-1], "wav")
				return commandResult{ExitCode: 0}, nil
			}

			usedModel = argValue(args, "-m")
			usedLanguage = argValue(args, "-l")
			base := argValue(args, "-of")
			mustWriteFile(t, base+".txt", "transcribed")
			return commandResult{ExitCode: 0}, nil
		},
	}

	pipeline := NewPipelineForTests("ffmpeg", "whisper.cpp", runner, os.MkdirTemp, os.RemoveAll, os.Stat)
	result, err := pipeline.Run(context.Background(), Request{
		InputPath: inputPath,
		ModelPath: modelDir,
		Language:  "en",
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantModel := filepath.Join(modelDir, "a-small.gguf")
	if usedModel != wantModel {
		t.Fatalf("used model = %q, want %q", usedModel, wantModel)
	}
	if usedLanguage != "en" {
		t.Fatalf("used language = %q, want en", usedLanguage)
	}
	if result.Transcript != "transcribed" {
		t.Fatalf("transcript = %q", result.Transcript)
	}
}

// TestPipelineRunWhisperFailureCleansTempDir checks failure cleanup path.
func TestPipelineRunWhisperFailureCleansTempDir(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "clip.mp4")
	modelPath := filepath.Join(root, "model.bin")
	outputDir := filepath.Join(root, "out")
	mustWriteFile(t, inputPath, "media")
	mustWriteFile(t, modelPath, "model")

	var tempDir string
	runner := &fakeRunner{
		run: func(ctx context.Context, name string, args ...string) (commandResult, error) {
			if name == "ffmpeg" {
				outPath := args[len(args)-1]
				tempDir = filepath.Dir(outPath)
				mustWriteFile(t, outPath, "wav")
				return commandResult{ExitCode: 0}, nil
			}
			return commandResult{
				Stderr:   "whisper failed",
				ExitCode: 1,
			}, errors.New("exit status 1")
		},
	}

	pipeline := NewPipelineForTests("ffmpeg", "whisper.cpp", runner, os.MkdirTemp, os.RemoveAll, os.Stat)
	_, err := pipeline.Run(context.Background(), Request{
		InputPath: inputPath,
		ModelPath: modelPath,
		Language:  "auto",
		OutputDir: outputDir,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var pErr *PipelineError
	if !errors.As(err, &pErr) {
		t.Fatalf("error type = %T, want *PipelineError", err)
	}
	if pErr.Stage != "transcribing" {
		t.Fatalf("stage = %s, want transcribing", pErr.Stage)
	}
	if pErr.CommandLog.Command != "whisper.cpp" {
		t.Fatalf("command = %q, want whisper.cpp", pErr.CommandLog.Command)
	}
	if pErr.CommandLog.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1", pErr.CommandLog.ExitCode)
	}
	if _, statErr := os.Stat(tempDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("temp dir should be removed on failure, stat err = %v", statErr)
	}
}

// TestPipelineRunRequiresModelPath checks validation for missing model path.
func TestPipelineRunRequiresModelPath(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "clip.mp3")
	outputDir := filepath.Join(root, "out")
	mustWriteFile(t, inputPath, "media")

	pipeline := NewPipelineForTests("ffmpeg", "whisper.cpp", &fakeRunner{}, os.MkdirTemp, os.RemoveAll, os.Stat)
	_, err := pipeline.Run(context.Background(), Request{
		InputPath: inputPath,
		OutputDir: outputDir,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var pErr *PipelineError
	if !errors.As(err, &pErr) {
		t.Fatalf("error type = %T, want *PipelineError", err)
	}
	if pErr.Stage != "transcribing" {
		t.Fatalf("stage = %s, want transcribing", pErr.Stage)
	}
}

// TestBuildFFmpegArgs verifies deterministic ffmpeg command arguments.
func TestBuildFFmpegArgs(t *testing.T) {
	args := buildFFmpegArgs("/in.mp4", "/tmp/out.wav")
	want := []string{
		"-hide_banner",
		"-nostdin",
		"-y",
		"-i", "/in.mp4",
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		"/tmp/out.wav",
	}

	if len(args) != len(want) {
		t.Fatalf("args len = %d, want %d", len(args), len(want))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

// TestBuildWhisperArgsAutoLanguage verifies no language flag for auto mode.
func TestBuildWhisperArgsAutoLanguage(t *testing.T) {
	args := buildWhisperArgs("/m.bin", "/audio.wav", "/out/base", "auto")
	if hasArg(args, "-l") {
		t.Fatalf("did not expect -l in args: %v", args)
	}
}

// TestBuildWhisperArgsFixedLanguage verifies language flag for fixed mode.
func TestBuildWhisperArgsFixedLanguage(t *testing.T) {
	args := buildWhisperArgs("/m.bin", "/audio.wav", "/out/base", "ru")
	if !hasArg(args, "-l") {
		t.Fatalf("expected -l in args: %v", args)
	}
	if got := argValue(args, "-l"); got != "ru" {
		t.Fatalf("language arg = %q, want ru", got)
	}
}

// mustWriteFile creates parent directory and writes file content.
func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

// argValue returns value for key-style CLI args.
func argValue(args []string, key string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key {
			return args[i+1]
		}
	}
	return ""
}

// hasArg reports whether args include the target flag.
func hasArg(args []string, key string) bool {
	for _, arg := range args {
		if arg == key {
			return true
		}
	}
	return false
}
