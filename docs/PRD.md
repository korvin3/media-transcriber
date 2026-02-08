## Product
Desktop app for local video/audio transcription using Wails (Go) + whisper.cpp + ffmpeg.

## Objective
Provide a fast, private, offline tool for personal use on macOS and Windows with minimal UI and reliable transcript export.

## Target User
Single user (you), technical enough to install app binaries/models, wants simple workflow over advanced editing.

## Scope
- Import media file (mp4, mov, mkv, mp3, wav, etc.).
- Preprocess audio via ffmpeg to whisper-friendly format (mono, 16kHz, WAV PCM).
- Run transcription with whisper.cpp (CLI first; Go binding optional later).
- Show progress/logs and allow cancel.
- Export transcript as .txt.
- Setup settings: models download menu, output folder.
- Drag/drop and file picker input.
- Validate input and dependencies on startup (ffmpeg, whisper.cpp, model exists).
- Queue size: one job at a time.
- Persist last-used settings locally.
- Open output folder after completion.

## Non-Functional Requirements
- Fully offline processing.
- Cross-platform support: macOS 13+ and Windows 10+.
- App should remain responsive during transcription.
- Clear error messages for missing binaries/model or failed conversion.

## Technical Approach
- Wails frontend for minimal UI (single-window workflow).
- Go backend orchestrates subprocess calls to ffmpeg and whisper.cpp.
- Job state machine: idle -> preprocessing -> transcribing -> exporting -> done/failed/cancelled.
- Structured logs saved per job.

## Success Criteria
- User can transcribe a 3 hours video end-to-end without manual CLI steps.
- Transcript files are generated correctly in selected output folder.
- Cancellation works without app freeze.
- Packaging works on both macOS and Windows.

## Step-by-Step Tasks Plan
### Phase 1 - Project Bootstrap
1. Initialize Wails app (Go backend + frontend shell) and base folder structure.
2. Add core modules: `internal/config`, `internal/jobs`, `internal/transcribe`.
3. Define app constants and shared types for job status and settings.
Done when app starts on macOS and Windows dev environments.

### Phase 2 - Dependency and Environment Validation
1. Implement startup checks for `ffmpeg`, `ffprobe`, and `whisper.cpp`.
2. Validate model path and output directory; show actionable errors.
3. Add a diagnostics panel in UI for dependency status.
Done when startup reports clear pass/fail status for all required tools.

### Phase 3 - Media Preprocessing Pipeline
1. Implement Go wrapper to run `ffmpeg` and convert input to mono, 16kHz WAV PCM.
2. Capture stdout/stderr logs and return structured errors.
3. Add temporary file management and cleanup policy.
Done when sample video/audio files are consistently converted.

### Phase 4 - Transcription Engine Integration
1. Implement `whisper.cpp` execution flow (CLI first).
2. Support language `auto` and fixed language option from settings.
3. Parse output and save transcript as `.txt`.
Done when converted audio produces correct transcript files.

### Phase 5 - Job Lifecycle and Cancellation
1. Build state machine: `idle -> preprocessing -> transcribing -> exporting -> done/failed/cancelled`.
2. Add single-job queue enforcement and cancel support.
3. Stream progress/log messages to frontend in real time.
Done when cancellation works safely and UI remains responsive.

### Phase 6 - Minimal UI Workflow
1. Build single-window flow: file picker/drag-drop, settings, start/cancel, logs, result.
2. Add button to open output folder after completion.
3. Persist last-used settings locally.
Done when end-to-end flow works without CLI usage.

### Phase 7 - Quality and Reliability
1. Add unit tests for config, state machine transitions, and command builders.
2. Add integration tests with mocked process runners.
3. Test failure paths: missing tools, bad model path, conversion/transcription errors.
Done when core pipeline and failure handling are covered by automated tests.

### Phase 8 - Packaging and Release Readiness
1. Build signed/packaged app for macOS and Windows using Wails build targets.
2. Run smoke tests on both platforms with a 3-hour media sample.
3. Write quick start docs for setup, model placement, and troubleshooting.
Done when success criteria are validated on both target OS versions.
