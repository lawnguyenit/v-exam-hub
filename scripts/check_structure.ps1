$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$violations = @()

$forbiddenRootGo = @(
  "config.go",
  "utils.go",
  "helpers.go",
  "storage.go",
  "session.go",
  "cors.go",
  "http_helpers.go"
)

foreach ($fileName in $forbiddenRootGo) {
  $path = Join-Path $root $fileName
  if (Test-Path -LiteralPath $path) {
    $violations += "Backend root must not contain $fileName. Move it into an internal package."
  }
}

$frontendRoot = Join-Path $root "frontend\src"
$forbiddenFrontendRoot = @(
  "config.ts",
  "utils.ts",
  "helpers.ts",
  "storage.ts"
)

foreach ($fileName in $forbiddenFrontendRoot) {
  $path = Join-Path $frontendRoot $fileName
  if (Test-Path -LiteralPath $path) {
    $violations += "Frontend src root must not contain $fileName. Move it into src/lib, src/api, or a feature folder."
  }
}

if ($violations.Count -gt 0) {
  Write-Error ($violations -join [Environment]::NewLine)
}

Write-Host "Project structure check passed."
