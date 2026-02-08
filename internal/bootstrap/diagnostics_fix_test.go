package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"media-transcriber/internal/domain"
)

// TestResolveModelDownloadPlanForModelFilePath ensures explicit model files are preserved.
func TestResolveModelDownloadPlanForModelFilePath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "ggml-model.bin")

	plan, err := resolveModelDownloadPlan(target)
	if err != nil {
		t.Fatalf("resolve plan: %v", err)
	}
	if plan.targetFile != target {
		t.Fatalf("targetFile = %s, want %s", plan.targetFile, target)
	}
	if plan.settingsPath != target {
		t.Fatalf("settingsPath = %s, want %s", plan.settingsPath, target)
	}
}

// TestResolveModelDownloadPlanForDirectory ensures folder paths download into default model file.
func TestResolveModelDownloadPlanForDirectory(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir model dir: %v", err)
	}

	plan, err := resolveModelDownloadPlan(modelDir)
	if err != nil {
		t.Fatalf("resolve plan: %v", err)
	}
	wantTarget := filepath.Join(modelDir, defaultWhisperModelFilename)
	if plan.targetFile != wantTarget {
		t.Fatalf("targetFile = %s, want %s", plan.targetFile, wantTarget)
	}
	if plan.settingsPath != modelDir {
		t.Fatalf("settingsPath = %s, want %s", plan.settingsPath, modelDir)
	}
}

// TestResolveModelDownloadPlanRejectsNonModelFile ensures invalid file paths are rejected.
func TestResolveModelDownloadPlanRejectsNonModelFile(t *testing.T) {
	root := t.TempDir()
	badFile := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(badFile, []byte("not a model"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := resolveModelDownloadPlan(badFile); err == nil {
		t.Fatal("expected error for non-model file path")
	}
}

// TestInstallOrFixOutputDirCreatesDirectory ensures output dir fix creates missing directories.
func TestInstallOrFixOutputDirCreatesDirectory(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "nested", "transcripts")

	settings := domain.Settings{
		ModelPath: "",
		OutputDir: outputDir,
		Language:  "auto",
	}
	fixed, changed, err := installOrFixOutputDir(settings)
	if err != nil {
		t.Fatalf("fix output dir: %v", err)
	}
	if changed {
		t.Fatal("expected settings to remain unchanged")
	}
	if fixed.OutputDir != outputDir {
		t.Fatalf("OutputDir = %s, want %s", fixed.OutputDir, outputDir)
	}
	if _, err := os.Stat(outputDir); err != nil {
		t.Fatalf("stat output dir: %v", err)
	}
}

// TestSelectWhisperWindowsAssetPrefersWhisperBinX64 validates preferred asset matching.
func TestSelectWhisperWindowsAssetPrefersWhisperBinX64(t *testing.T) {
	release := githubRelease{
		TagName: "v1.0.0",
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "whisper-bin-arm64.zip", URL: "https://example.com/arm64.zip"},
			{Name: "whisper-bin-x64.zip", URL: "https://example.com/x64.zip"},
		},
	}

	url, name, err := selectWhisperWindowsAsset(release)
	if err != nil {
		t.Fatalf("select asset: %v", err)
	}
	if url != "https://example.com/x64.zip" {
		t.Fatalf("url = %s, want x64 asset", url)
	}
	if name != "whisper-bin-x64.zip" {
		t.Fatalf("name = %s, want whisper-bin-x64.zip", name)
	}
}

// TestSelectWhisperWindowsAssetSupportsGenericWindowsPattern validates fallback matching.
func TestSelectWhisperWindowsAssetSupportsGenericWindowsPattern(t *testing.T) {
	release := githubRelease{
		TagName: "v1.0.0",
		Assets: []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{
			{Name: "whisper-win-x64-cuda.zip", URL: "https://example.com/win-x64.zip"},
		},
	}

	url, _, err := selectWhisperWindowsAsset(release)
	if err != nil {
		t.Fatalf("select asset: %v", err)
	}
	if url != "https://example.com/win-x64.zip" {
		t.Fatalf("url = %s, want win-x64 asset", url)
	}
}

// TestIsWithinBaseDirRejectsTraversal validates archive path traversal guard.
func TestIsWithinBaseDirRejectsTraversal(t *testing.T) {
	base := filepath.Join("C:\\", "tmp", "root")
	target := filepath.Join(base, "..", "escape.txt")
	if isWithinBaseDir(base, target) {
		t.Fatal("expected traversal target to be rejected")
	}
}
