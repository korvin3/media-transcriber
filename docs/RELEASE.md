# Release Packaging (Phase 8)

## Цель
Собрать и подписать desktop-артефакты для macOS и Windows через Wails build targets.

## CI (GitHub Actions)
В репозитории настроен workflow: `.github/workflows/release-build.yml`.

- `workflow_dispatch`: ручной запуск matrix-сборки (`macos-14`, `windows-2022`).
- `push` с тегом `v*`: matrix-сборка + публикация GitHub Release.
- На release шаге формируются `SHA256SUMS.txt` и прикладываются артефакты из `build/bin/`.

Рекомендуемый формат тегов: `v0.1.0`, `v0.2.3`.

## Предварительные требования
- Установлен `wails` CLI.
- Настроены toolchain зависимости Wails для целевой ОС.
- При необходимости подписи заполнены переменные из `scripts/release/env.example`.

## Артефакты
Ожидаемые файлы находятся в `build/bin/`:
- macOS: `.app`, `.dmg`
- Windows: `.exe` (приложение и/или installer)

## macOS
1. Откройте терминал на macOS.
2. Экспортируйте переменные подписи (опционально):
   - `MAC_SIGN_IDENTITY`
   - `APPLE_ID`, `APPLE_TEAM_ID`, `APPLE_APP_PASSWORD` (для notarization)
3. Запустите:
```bash
./scripts/release/build-macos.sh
```

Что делает скрипт:
- `wails build -clean -platform darwin/universal`
- codesign `.app` и `.dmg` (если указан identity)
- notarytool submit + staple (если заданы Apple credentials)

## Windows
1. Для обычной сборки (без подписи) запустите:
```bash
./scripts/release/build-windows.sh
```
2. Для подписи на Windows откройте PowerShell.
3. Экспортируйте переменные подписи (опционально):
   - `WIN_SIGN_CERT_FILE`
   - `WIN_SIGN_CERT_PASSWORD`
   - `WIN_SIGN_TIMESTAMP_URL`
4. Запустите:
```powershell
pwsh -File .\scripts\release\build-windows.ps1
```

Что делает `build-windows.sh`:
- `wails build -clean -platform windows/amd64`
- если `wails build` падает, выполняет fallback:
  - `GOOS=windows GOARCH=amd64 go build -o build/bin/media-transcriber.exe .`
  - этот режим не делает platform packaging/signing, только сборку `.exe`

Что делает `build-windows.ps1`:
- `wails build -clean -platform windows/amd64`
- подпись всех `.exe` через `signtool` (если заданы переменные)

## Проверка
- Убедитесь, что артефакты появились в `build/bin/`.
- На подписанных артефактах проверьте валидность подписи средствами ОС.
