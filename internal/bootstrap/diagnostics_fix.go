package bootstrap

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"media-transcriber/internal/config"
	"media-transcriber/internal/domain"
)

const (
	defaultWhisperModelFilename = "ggml-base.en.bin"
	defaultWhisperModelURL      = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin"

	installCommandTimeout = 45 * time.Minute
	modelDownloadTimeout  = 45 * time.Minute
	downloadToolTimeout   = 30 * time.Minute
)

type installOption struct {
	manager  string
	commands [][]string
}

type modelDownloadPlan struct {
	targetFile   string
	settingsPath string
}

// InstallOrFixDiagnostic applies an OS-specific remediation for one failed diagnostic item.
func (a *App) InstallOrFixDiagnostic(itemID string) (domain.DiagnosticReport, error) {
	if a.Store == nil {
		return domain.DiagnosticReport{}, fmt.Errorf("settings store is not configured")
	}

	id := strings.TrimSpace(itemID)
	if id == "" {
		return domain.DiagnosticReport{}, fmt.Errorf("diagnostic item id is required")
	}

	settings, err := a.Store.Load()
	if err != nil {
		return domain.DiagnosticReport{}, fmt.Errorf("load settings: %w", err)
	}
	settings = normalizeSettings(settings)

	settingsChanged := false
	var fixErr error

	switch id {
	case "tool_ffmpeg", "tool_ffprobe":
		fixErr = installFFmpegForCurrentOS()
	case "tool_whisper.cpp":
		fixErr = installWhisperForCurrentOS()
	case "model_path":
		settings, settingsChanged, fixErr = installOrFixModelPath(settings)
	case "output_dir":
		settings, settingsChanged, fixErr = installOrFixOutputDir(settings)
	default:
		return domain.DiagnosticReport{}, fmt.Errorf("unsupported diagnostic item id: %s", id)
	}

	if settingsChanged {
		if saveErr := a.Store.Save(settings); saveErr != nil {
			report := a.refreshDiagnosticsFromSettings(settings)
			return report, fmt.Errorf("save settings after fix: %w", saveErr)
		}
	}

	report := a.refreshDiagnosticsFromSettings(settings)
	if fixErr != nil {
		return report, fixErr
	}
	return report, nil
}

func (a *App) refreshDiagnosticsFromSettings(settings domain.Settings) domain.DiagnosticReport {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Settings = settings
	if a.checker != nil {
		a.Diagnostics = a.checker.Run(settings)
	}
	return a.Diagnostics
}

func ensureLocalBinOnPATH(homeDir string) error {
	binDir := localBinDir(homeDir)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	current := os.Getenv("PATH")
	entries := filepath.SplitList(current)
	for _, entry := range entries {
		if filepath.Clean(entry) == filepath.Clean(binDir) {
			return nil
		}
	}

	if current == "" {
		return os.Setenv("PATH", binDir)
	}
	return os.Setenv("PATH", binDir+string(os.PathListSeparator)+current)
}

func localBinDir(homeDir string) string {
	return filepath.Join(homeDir, ".media-transcriber", "bin")
}

func localModelsDir(homeDir string) string {
	return filepath.Join(homeDir, ".media-transcriber", "models")
}

func installFFmpegForCurrentOS() error {
	options := []installOption{}

	switch goruntime.GOOS {
	case "windows":
		options = []installOption{
			{
				manager: "winget",
				commands: [][]string{
					{"winget", "install", "--id", "Gyan.FFmpeg", "--exact", "--accept-source-agreements", "--accept-package-agreements"},
				},
			},
			{
				manager: "choco",
				commands: [][]string{
					{"choco", "install", "ffmpeg", "-y"},
				},
			},
			{
				manager: "scoop",
				commands: [][]string{
					{"scoop", "install", "ffmpeg"},
				},
			},
		}
	case "darwin":
		options = []installOption{
			{
				manager: "brew",
				commands: [][]string{
					{"brew", "install", "ffmpeg"},
				},
			},
		}
	default:
		options = []installOption{
			{
				manager: "apt-get",
				commands: [][]string{
					{"apt-get", "update"},
					{"apt-get", "install", "-y", "ffmpeg"},
				},
			},
			{
				manager: "dnf",
				commands: [][]string{
					{"dnf", "install", "-y", "ffmpeg"},
				},
			},
			{
				manager: "pacman",
				commands: [][]string{
					{"pacman", "-Sy", "--noconfirm", "ffmpeg"},
				},
			},
			{
				manager: "zypper",
				commands: [][]string{
					{"zypper", "install", "-y", "ffmpeg"},
				},
			},
			{
				manager: "brew",
				commands: [][]string{
					{"brew", "install", "ffmpeg"},
				},
			},
		}
	}

	if err := runFirstSuccessfulInstall(options); err != nil {
		return fmt.Errorf("install ffmpeg/ffprobe: %w", err)
	}
	if err := requireToolsOnPath("ffmpeg", "ffprobe"); err != nil {
		return fmt.Errorf("verify ffmpeg/ffprobe on PATH: %w", err)
	}
	return nil
}

func installWhisperForCurrentOS() error {
	if err := requireToolsOnPath("whisper.cpp"); err == nil {
		return nil
	}
	if err := createWhisperAlias(); err == nil {
		if err := requireToolsOnPath("whisper.cpp"); err == nil {
			return nil
		}
	}

	options := []installOption{}

	switch goruntime.GOOS {
	case "windows":
		options = []installOption{
			{
				manager: "winget",
				commands: [][]string{
					{"winget", "install", "--id", "ggerganov.whisper.cpp", "--exact", "--accept-source-agreements", "--accept-package-agreements"},
				},
			},
			{
				manager: "winget",
				commands: [][]string{
					{"winget", "install", "--name", "whisper.cpp", "--accept-source-agreements", "--accept-package-agreements"},
				},
			},
			{
				manager: "choco",
				commands: [][]string{
					{"choco", "install", "whispercpp", "-y"},
				},
			},
			{
				manager: "scoop",
				commands: [][]string{
					{"scoop", "install", "whisper-cpp"},
				},
			},
		}
	case "darwin":
		options = []installOption{
			{
				manager: "brew",
				commands: [][]string{
					{"brew", "install", "whisper-cpp"},
				},
			},
		}
	default:
		options = []installOption{
			{
				manager: "apt-get",
				commands: [][]string{
					{"apt-get", "update"},
					{"apt-get", "install", "-y", "whisper-cpp"},
				},
			},
			{
				manager: "apt-get",
				commands: [][]string{
					{"apt-get", "update"},
					{"apt-get", "install", "-y", "whisper.cpp"},
				},
			},
			{
				manager: "dnf",
				commands: [][]string{
					{"dnf", "install", "-y", "whisper-cpp"},
				},
			},
			{
				manager: "pacman",
				commands: [][]string{
					{"pacman", "-Sy", "--noconfirm", "whisper.cpp"},
				},
			},
			{
				manager: "zypper",
				commands: [][]string{
					{"zypper", "install", "-y", "whisper-cpp"},
				},
			},
			{
				manager: "brew",
				commands: [][]string{
					{"brew", "install", "whisper-cpp"},
				},
			},
		}
	}

	installErr := runFirstSuccessfulInstall(options)
	if installErr == nil {
		if err := requireToolsOnPath("whisper.cpp"); err == nil {
			return nil
		}
	}

	if goruntime.GOOS == "windows" {
		if err := installWhisperWindowsFromGithubRelease(); err == nil {
			if err := requireToolsOnPath("whisper.cpp"); err == nil {
				return nil
			}
		} else if installErr != nil {
			installErr = fmt.Errorf("%v | release fallback: %w", installErr, err)
		} else {
			installErr = fmt.Errorf("release fallback: %w", err)
		}
	}

	if err := createWhisperAlias(); err != nil {
		if installErr != nil {
			return fmt.Errorf("install whisper.cpp failed: %v | alias creation failed: %w", installErr, err)
		}
		return fmt.Errorf("create whisper.cpp command alias: %w", err)
	}

	if err := requireToolsOnPath("whisper.cpp"); err != nil {
		if installErr != nil {
			return fmt.Errorf("install whisper.cpp failed: %v | verify whisper.cpp on PATH: %w", installErr, err)
		}
		return fmt.Errorf("verify whisper.cpp on PATH: %w", err)
	}
	return nil
}

func runFirstSuccessfulInstall(options []installOption) error {
	if len(options) == 0 {
		return fmt.Errorf("no install commands configured for OS %s", goruntime.GOOS)
	}

	errorsByManager := make([]string, 0, len(options))
	atLeastOneManager := false

	for _, option := range options {
		if !commandAvailable(option.manager) {
			continue
		}
		atLeastOneManager = true
		if err := runInstallCommands(option.commands); err == nil {
			return nil
		} else {
			errorsByManager = append(errorsByManager, fmt.Sprintf("%s: %v", option.manager, err))
		}
	}

	if !atLeastOneManager {
		return fmt.Errorf("no supported package manager found for %s", goruntime.GOOS)
	}
	return fmt.Errorf(strings.Join(errorsByManager, " | "))
}

func runInstallCommands(commands [][]string) error {
	for _, command := range commands {
		if err := runCommandWithPossibleElevation(command); err != nil {
			return err
		}
	}
	return nil
}

func runCommandWithPossibleElevation(command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("empty command")
	}

	candidates := [][]string{command}
	if goruntime.GOOS == "linux" && requiresElevation(command[0]) {
		if commandAvailable("pkexec") {
			candidates = append(candidates, append([]string{"pkexec"}, command...))
		}
		if commandAvailable("sudo") {
			candidates = append(candidates, append([]string{"sudo", "-n"}, command...))
		}
	}

	attemptErrors := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if err := runCommand(candidate[0], candidate[1:]...); err == nil {
			return nil
		} else {
			attemptErrors = append(attemptErrors, err.Error())
		}
	}

	return fmt.Errorf(strings.Join(attemptErrors, " | "))
}

func runCommand(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), installCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%s timed out after %s", formatCommand(name, args), installCommandTimeout)
	}

	trimmed := strings.TrimSpace(string(output))
	if len(trimmed) > 500 {
		trimmed = trimmed[:500] + "..."
	}
	if trimmed == "" {
		return fmt.Errorf("%s failed: %w", formatCommand(name, args), err)
	}
	return fmt.Errorf("%s failed: %w (%s)", formatCommand(name, args), err, trimmed)
}

func formatCommand(name string, args []string) string {
	parts := append([]string{name}, args...)
	return strings.Join(parts, " ")
}

func requiresElevation(manager string) bool {
	switch manager {
	case "apt-get", "dnf", "pacman", "zypper":
		return true
	default:
		return false
	}
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func requireToolsOnPath(names ...string) error {
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing tools on PATH: %s", strings.Join(missing, ", "))
	}
	return nil
}

func createWhisperAlias() error {
	if _, err := exec.LookPath("whisper.cpp"); err == nil {
		return nil
	}

	candidates := []string{"whisper-cli", "whisper", "whisper-cpp", "main"}
	var sourcePath string
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			sourcePath = path
			break
		}
	}
	if sourcePath == "" {
		return fmt.Errorf("no compatible whisper executable found (tried: %s)", strings.Join(candidates, ", "))
	}

	return createWhisperAliasFromExecutable(sourcePath)
}

func createWhisperAliasFromExecutable(sourcePath string) error {
	if strings.TrimSpace(sourcePath) == "" {
		return fmt.Errorf("source executable path is empty")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home: %w", err)
	}

	if err := ensureLocalBinOnPATH(homeDir); err != nil {
		return err
	}

	binDir := localBinDir(homeDir)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create local bin directory: %w", err)
	}

	if goruntime.GOOS == "windows" {
		aliasPath := filepath.Join(binDir, "whisper.cpp.cmd")
		content := fmt.Sprintf("@echo off\r\n\"%s\" %%*\r\n", sourcePath)
		if err := os.WriteFile(aliasPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write whisper alias file: %w", err)
		}
		return nil
	}

	aliasPath := filepath.Join(binDir, "whisper.cpp")
	escaped := strings.ReplaceAll(sourcePath, "\"", "\\\"")
	content := fmt.Sprintf("#!/usr/bin/env sh\nexec \"%s\" \"$@\"\n", escaped)
	if err := os.WriteFile(aliasPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write whisper alias script: %w", err)
	}
	return nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func installWhisperWindowsFromGithubRelease() error {
	release, err := fetchLatestWhisperRelease()
	if err != nil {
		return err
	}

	assetURL, assetName, err := selectWhisperWindowsAsset(release)
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home: %w", err)
	}

	installDir := filepath.Join(homeDir, ".media-transcriber", "tools", "whisper.cpp", release.TagName)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create whisper install directory: %w", err)
	}

	zipPath := filepath.Join(installDir, assetName)
	if err := downloadURLToFile(zipPath, assetURL, downloadToolTimeout); err != nil {
		return fmt.Errorf("download release asset: %w", err)
	}

	executablePath, err := extractWhisperWindowsZip(zipPath, installDir)
	if err != nil {
		return fmt.Errorf("extract whisper release asset: %w", err)
	}

	if err := createWhisperAliasFromExecutable(executablePath); err != nil {
		return err
	}
	return nil
}

func fetchLatestWhisperRelease() (githubRelease, error) {
	urls := []string{
		"https://api.github.com/repos/ggml-org/whisper.cpp/releases/latest",
		"https://api.github.com/repos/ggerganov/whisper.cpp/releases/latest",
	}

	var lastErr error
	for _, url := range urls {
		release, err := fetchGithubRelease(url)
		if err == nil {
			return release, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown error")
	}
	return githubRelease{}, fmt.Errorf("fetch latest whisper.cpp release metadata: %w", lastErr)
}

func fetchGithubRelease(url string) (githubRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadToolTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return githubRelease{}, fmt.Errorf("build release metadata request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "media-transcriber")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("request release metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("release metadata request returned %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("decode release metadata: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, fmt.Errorf("release metadata did not include a tag name")
	}
	return release, nil
}

func selectWhisperWindowsAsset(release githubRelease) (url string, name string, err error) {
	if len(release.Assets) == 0 {
		return "", "", fmt.Errorf("release %s has no assets", release.TagName)
	}

	selectByPredicate := func(predicate func(string) bool) (string, string, bool) {
		for _, asset := range release.Assets {
			assetName := strings.ToLower(strings.TrimSpace(asset.Name))
			if !predicate(assetName) {
				continue
			}
			if strings.TrimSpace(asset.URL) == "" {
				continue
			}
			return asset.URL, asset.Name, true
		}
		return "", "", false
	}

	if url, name, ok := selectByPredicate(func(assetName string) bool {
		return strings.Contains(assetName, "whisper-bin-x64.zip")
	}); ok {
		return url, name, nil
	}

	if url, name, ok := selectByPredicate(func(assetName string) bool {
		return strings.HasSuffix(assetName, ".zip") &&
			(strings.Contains(assetName, "win") || strings.Contains(assetName, "windows")) &&
			strings.Contains(assetName, "x64")
	}); ok {
		return url, name, nil
	}

	return "", "", fmt.Errorf("release %s does not contain a supported Windows x64 zip asset", release.TagName)
}

func downloadURLToFile(destinationPath string, sourceURL string, timeout time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("prepare destination directory: %w", err)
	}

	tmpPath := destinationPath + ".download"
	if err := os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale temp file: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "media-transcriber")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}

	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write destination file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close destination file: %w", closeErr)
	}

	if err := os.Remove(destinationPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("remove old destination file: %w", err)
	}
	if err := os.Rename(tmpPath, destinationPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("move downloaded file into place: %w", err)
	}

	return nil
}

func extractWhisperWindowsZip(zipPath string, extractDir string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var executablePath string

	for _, file := range reader.File {
		if file == nil {
			continue
		}
		cleanName := filepath.Clean(file.Name)
		if cleanName == "." || cleanName == "" {
			continue
		}
		targetPath := filepath.Join(extractDir, cleanName)
		if !isWithinBaseDir(extractDir, targetPath) {
			return "", fmt.Errorf("zip contains invalid path: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return "", err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", err
		}

		src, err := file.Open()
		if err != nil {
			return "", err
		}

		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			_ = src.Close()
			return "", err
		}

		_, copyErr := io.Copy(dst, src)
		srcCloseErr := src.Close()
		dstCloseErr := dst.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if srcCloseErr != nil {
			return "", srcCloseErr
		}
		if dstCloseErr != nil {
			return "", dstCloseErr
		}

		baseName := strings.ToLower(filepath.Base(targetPath))
		if baseName == "whisper-cli.exe" || baseName == "main.exe" || baseName == "whisper.cpp.exe" {
			executablePath = targetPath
		}
	}

	if strings.TrimSpace(executablePath) == "" {
		return "", fmt.Errorf("extracted archive does not contain whisper executable (whisper-cli.exe/main.exe)")
	}
	return executablePath, nil
}

func isWithinBaseDir(baseDir string, targetPath string) bool {
	baseClean := filepath.Clean(baseDir)
	targetClean := filepath.Clean(targetPath)
	relative, err := filepath.Rel(baseClean, targetClean)
	if err != nil {
		return false
	}
	return relative == "." || (!strings.HasPrefix(relative, "..") && relative != "")
}

func installOrFixModelPath(settings domain.Settings) (domain.Settings, bool, error) {
	plan, err := resolveModelDownloadPlan(settings.ModelPath)
	if err != nil {
		return settings, false, err
	}

	if err := downloadFile(plan.targetFile, defaultWhisperModelURL); err != nil {
		return settings, false, fmt.Errorf("download model: %w", err)
	}

	changed := strings.TrimSpace(settings.ModelPath) != plan.settingsPath
	settings.ModelPath = plan.settingsPath
	return settings, changed, nil
}

func resolveModelDownloadPlan(modelPath string) (modelDownloadPlan, error) {
	trimmed := strings.TrimSpace(modelPath)
	if trimmed == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return modelDownloadPlan{}, fmt.Errorf("resolve user home: %w", err)
		}
		dir := localModelsDir(homeDir)
		return modelDownloadPlan{
			targetFile:   filepath.Join(dir, defaultWhisperModelFilename),
			settingsPath: dir,
		}, nil
	}

	info, err := os.Stat(trimmed)
	if err == nil {
		if info.IsDir() {
			return modelDownloadPlan{
				targetFile:   filepath.Join(trimmed, defaultWhisperModelFilename),
				settingsPath: trimmed,
			}, nil
		}

		ext := strings.ToLower(filepath.Ext(trimmed))
		if ext == ".bin" || ext == ".gguf" {
			return modelDownloadPlan{
				targetFile:   trimmed,
				settingsPath: trimmed,
			}, nil
		}

		return modelDownloadPlan{}, fmt.Errorf("model path points to a non-model file: %s", trimmed)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return modelDownloadPlan{}, fmt.Errorf("check model path: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(trimmed))
	if ext == ".bin" || ext == ".gguf" {
		return modelDownloadPlan{
			targetFile:   trimmed,
			settingsPath: trimmed,
		}, nil
	}

	return modelDownloadPlan{
		targetFile:   filepath.Join(trimmed, defaultWhisperModelFilename),
		settingsPath: trimmed,
	}, nil
}

func downloadFile(destinationPath string, sourceURL string) error {
	return downloadURLToFile(destinationPath, sourceURL, modelDownloadTimeout)
}

func installOrFixOutputDir(settings domain.Settings) (domain.Settings, bool, error) {
	outputDir := strings.TrimSpace(settings.OutputDir)
	changed := false
	if outputDir == "" {
		outputDir = config.DefaultSettings().OutputDir
		settings.OutputDir = outputDir
		changed = true
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return settings, changed, fmt.Errorf("create output directory %s: %w", outputDir, err)
	}

	return settings, changed, nil
}
