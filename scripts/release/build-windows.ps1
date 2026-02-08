param(
  [string]$Platform = "windows/amd64",
  [string]$AppName = "media-transcriber"
)

$ErrorActionPreference = "Stop"
Set-Location (Resolve-Path "$PSScriptRoot/../..")

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
  throw "wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

Write-Host "==> Building $AppName for $Platform"
wails build -clean -platform $Platform

$binDir = Join-Path (Get-Location) "build/bin"
$artifacts = Get-ChildItem -Path $binDir -Filter "*.exe" -File
if ($artifacts.Count -eq 0) {
  throw "No .exe artifacts found in $binDir"
}

$signCert = $env:WIN_SIGN_CERT_FILE
$signPass = $env:WIN_SIGN_CERT_PASSWORD
$tsUrl = $env:WIN_SIGN_TIMESTAMP_URL

if ($signCert -and $signPass -and $tsUrl) {
  if (-not (Get-Command signtool.exe -ErrorAction SilentlyContinue)) {
    throw "signtool.exe not found in PATH"
  }

  foreach ($file in $artifacts) {
    Write-Host "==> Signing $($file.FullName)"
    signtool.exe sign /fd SHA256 /f $signCert /p $signPass /tr $tsUrl /td SHA256 $file.FullName
  }
}

Write-Host "==> Windows release artifacts"
Get-ChildItem -Path $binDir | Format-Table Name,Length,LastWriteTime
