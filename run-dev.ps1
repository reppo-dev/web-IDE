# ============================================================
#  NexusAI - One-command dev launcher for Windows
#
#  Why this file exists:
#  config.LoadConfig() reads os.Getenv() only. There is no
#  godotenv loader in the Go code, so a .env file sitting on
#  disk is silently ignored. This script reads .env into the
#  process environment, then starts the daemon and the frontend.
#
#  Usage:  .\run-dev.ps1
# ============================================================

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$EnvFile = Join-Path $Root ".env"

if (-Not (Test-Path $EnvFile)) {
    Write-Host "ERROR: .env not found at $EnvFile" -ForegroundColor Red
    Write-Host "Copy .env.example to .env and fill in your API keys." -ForegroundColor Yellow
    exit 1
}

# --- Load .env into the current process environment ---
Get-Content $EnvFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { return }

    $idx = $line.IndexOf("=")
    if ($idx -lt 1) { return }

    $key   = $line.Substring(0, $idx).Trim()
    $value = $line.Substring($idx + 1).Trim().Trim('"')

    [Environment]::SetEnvironmentVariable($key, $value, "Process")
}

Write-Host ""
Write-Host "  NexusAI - Environment loaded" -ForegroundColor Cyan
Write-Host "  -----------------------------------------------"
Write-Host "  Provider BaseURL : $env:OPENAI_BASE_URL"
Write-Host "  Model            : $env:NEXUS_MODEL"

$keyPreview = if ($env:OPENAI_API_KEY) {
    $k = $env:OPENAI_API_KEY
    if ($k.Length -gt 12) { $k.Substring(0, 8) + "..." + $k.Substring($k.Length - 4) } else { "(set)" }
} else { "(EMPTY - requests will fail)" }

Write-Host "  API Key          : $keyPreview"
Write-Host "  Mode             : $env:NEXUS_MODE"
Write-Host "  Port             : $env:NEXUS_PORT"
Write-Host ""

if (-not $env:OPENAI_API_KEY) {
    Write-Host "WARNING: OPENAI_API_KEY is empty - the swarm cannot call any model." -ForegroundColor Red
    exit 1
}

# --- Preflight checks ---
foreach ($tool in @("go", "node", "npm")) {
    if (-Not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        Write-Host "ERROR: '$tool' is not on PATH. Install it and retry." -ForegroundColor Red
        exit 1
    }
}

Write-Host "  go   : $(go version)" -ForegroundColor DarkGray
Write-Host "  node : $(node -v)" -ForegroundColor DarkGray
Write-Host ""

# --- Start backend (hidden window, logs to backend\nexusd.log) ---
$backend = Join-Path $Root "backend"
$frontend = Join-Path $Root "frontend"
$logPath = Join-Path $backend "nexusd.log"

Write-Host "  Starting nexusd daemon on :$env:NEXUS_PORT ..." -ForegroundColor Yellow
Start-Process -FilePath "go" `
    -ArgumentList "run", "./cmd/nexusd" `
    -WorkingDirectory $backend `
    -RedirectStandardOutput $logPath `
    -RedirectStandardError (Join-Path $backend "nexusd.err.log") `
    -WindowStyle Hidden

# Wait for the health endpoint to come up before starting Vite
$ready = $false
for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 500
    try {
        Invoke-RestMethod -Uri "http://localhost:$env:NEXUS_PORT/health" -TimeoutSec 2 | Out-Null
        $ready = $true
        break
    } catch { }
}

if ($ready) {
    Write-Host "  Backend is healthy." -ForegroundColor Green
} else {
    Write-Host "  Backend did not answer /health in 20s. Check $logPath" -ForegroundColor Red
}

Write-Host "  Starting Vite dev server..." -ForegroundColor Yellow
Write-Host ""
Write-Host "  Open http://localhost:3000" -ForegroundColor Cyan
Write-Host "  Press Ctrl+C to stop." -ForegroundColor DarkGray
Write-Host ""

Set-Location $frontend
npm run dev