# Media Transcriber

## Быстрый старт

1. Установите `ffmpeg`, `ffprobe`, `whisper.cpp` и добавьте в `PATH`.
2. Положите модель `.bin`/`.gguf` в `~/.media-transcriber/models` (или укажите свой путь в настройках).
3. Запустите приложение:
   - `go run .` или
   - `wails dev`

Подробно: `docs/QUICKSTART.md`.

## Как работает приложение

1. Запускается `main.go`, создаётся `bootstrap.App` через `New()`.
2. В `New()` загружаются настройки из `~/.media-transcriber/settings.json` (или берутся дефолтные), создаются менеджер задач, пайплайн транскрибации, диагностика окружения и шина событий.
3. `App.Run()` запускает Wails-окно, подключает фронтенд и биндинг backend-методов (`GetDiagnostics`, `StartTranscription`, `CancelTranscription` и др.).
4. В `Startup` сохраняется runtime-контекст Wails для push-событий через `runtime.EventsEmit("job:event", ...)`.
5. Фронтенд запрашивает диагностику и подписывается на поток `job:event`.

### Запуск транскрибации

6. `StartTranscription(inputPath)` перечитывает настройки, создаёт `jobID`, проверяет, что нет активной задачи, переводит задачу в `preprocessing` и запускает обработку в отдельной goroutine.
7. `runTranscriptionJob(...)` вызывает `pipeline.Run(...)`, передавая колбэки стадий и логов.
8. Стадия `preprocessing`: `ffmpeg` конвертирует входной файл в WAV (`mono`, `16kHz`, `pcm_s16le`) во временной директории.
9. Стадия `transcribing`: определяется путь к модели (`.bin`/`.gguf`), запускается `whisper.cpp`, создаётся `.txt` в выходной папке.
10. Стадия `exporting`: читается итоговый `.txt`, формируется `Result` (путь, текст, логи), временные файлы очищаются.

### Завершение

11. При успехе статус становится `done`, отправляется `result`-событие с `TextPath`.
12. При ошибке статус `failed`, публикуются детали ошибки и лог команды.
13. При отмене `CancelTranscription()` отменяет контекст, статус становится `cancelled`, UI получает событие отмены.

## Как работает шина событий

1. В `internal/jobs/events.go` определены:
   - `EventType`: `status`, `log`, `result`, `error`
   - `Event`: полезная нагрузка (`JobID`, `Status`, `Message`, `Command`, `TextPath` и т.д.) + `Seq` и `Timestamp`.
2. `EventBus` хранит события в памяти и защищён `sync.RWMutex`.
3. `Publish(...)` увеличивает `nextSeq`, присваивает `Seq`, при необходимости ставит `Timestamp`, сохраняет событие.
4. `Since(seq)` возвращает только новые события (`Seq > seq`) для инкрементального чтения.
5. Буфер ограничен `maxEvents`; старые события обрезаются при переполнении.
6. В `internal/bootstrap/app.go` метод `publishEvent(...)`:
   - сохраняет событие в `EventBus` (история),
   - пушит его в Wails runtime через `EventsEmit("job:event", ...)` (live-канал).
7. Фронтенд подписывается через `window.runtime.EventsOn("job:event", ...)` и получает события в реальном времени.
8. Дополнительно backend-метод `JobEvents(sinceSeq)` позволяет догрузить историю по `Seq`.

## Release и smoke test

- Packaging/signing:
  - macOS: `./scripts/release/build-macos.sh`
  - Windows build (bash): `./scripts/release/build-windows.sh`
  - Windows signing (PowerShell): `pwsh -File .\\scripts\\release\\build-windows.ps1`
    - если `wails build` падает локально, скрипт автоматически делает fallback `go build` и выдаёт `.exe`
- Универсальный запуск по ОС: `./scripts/release/build-all.sh`
- CI workflow: `.github/workflows/release-build.yml`
  - ручной запуск: `workflow_dispatch`
  - auto release: push тега `v*`
- Протокол smoke-теста (3 часа): `docs/SMOKE_TEST.md`
- Шаблон отчёта: `docs/smoke-results/TEMPLATE.md`
- Создать новый отчёт: `./scripts/smoke/new-run.sh`
- Подробная release-документация: `docs/RELEASE.md`
