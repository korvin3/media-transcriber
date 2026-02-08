# Repository Guidelines

## Project Structure & Module Organization
This project is a Wails + Go desktop app for local media transcription.
- `main.go`, `cmd/app/main.go`: application entrypoints.
- `internal/bootstrap/`: app wiring, Wails lifecycle, runtime events.
- `internal/transcribe/`: ffmpeg preprocessing + whisper.cpp transcription pipeline.
- `internal/jobs/`: job state machine and event bus.
- `internal/diagnostics/`: startup checks for tools and paths.
- `internal/config/`: default settings and JSON persistence.
- `internal/domain/`: shared types (`JobStatus`, diagnostics models).
- `frontend/`: minimal UI (`index.html`) subscribed to Wails events.
- `docs/PRD.md`: product scope and implementation phases.

## Build, Test, and Development Commands
Run from repo root:
- `go test ./...`: run unit tests across all packages.
- `go test -race ./...`: check concurrency behavior (recommended before PR).
- `go run .`: start the desktop app via Wails bootstrap.
- `wails dev`: run Wails live development mode (requires Wails CLI).
- `wails build`: create platform binaries/installers.
- `gofmt -w .`: format all Go files.

## Coding Style & Naming Conventions
- Use idiomatic Go and keep files `gofmt`-clean.
- Prefer focused files with explicit responsibilities (e.g., `manager.go`, `events.go`).
- Keep frontend logic simple and framework-free unless a migration is planned.

## Testing Guidelines
- Use Go’s `testing` package with colocated `*_test.go` files.
- Prefer table-driven tests for validators, transitions, and command args.
- Cover success, failure, and cancellation paths for pipeline/job flow.
- For async behaviors, assert eventual state with bounded waits.

## Security & Configuration Tips
- Do not commit model binaries, local media files, or generated transcripts.
- Validate tool paths (`ffmpeg`, `ffprobe`, `whisper.cpp`) before job start.
- Log actionable errors, but avoid leaking sensitive file contents in logs.
