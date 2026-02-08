# Quick Start

## 1. Установите зависимости
Нужны бинарники в `PATH`:
- `ffmpeg`
- `ffprobe`
- `whisper.cpp`

Также нужен файл модели `whisper.cpp` (`.bin` или `.gguf`).

## 2. Запустите приложение
Для разработки:
```bash
go run .
```
Или через Wails:
```bash
wails dev
```

## 3. Настройте модель и выходную папку
При первом запуске используются дефолты:
- model path: `~/.media-transcriber/models`
- output dir: `~/Documents/Transcripts`

Положите модель в эту директорию или укажите путь к файлу модели.

## 4. Выполните первую транскрибацию
1. Выберите медиафайл.
2. Нажмите Start.
3. Дождитесь статуса `done`.
4. Проверьте `.txt` в output directory.

## Troubleshooting
- `Tool not found in PATH`: добавьте бинарник в `PATH` и перезапустите приложение.
- `No model files found`: поместите `.bin`/`.gguf` в model path.
- `Output directory is not writable`: выберите директорию с правами записи.
- `ffmpeg audio conversion failed`: проверьте входной файл и поддержку кодека.
- `whisper.cpp transcription failed`: проверьте совместимость модели с версией `whisper.cpp`.

## Полезные документы
- Release packaging: `docs/RELEASE.md`
- Smoke protocol: `docs/SMOKE_TEST.md`
