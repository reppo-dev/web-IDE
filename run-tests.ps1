# ============================================================
#  اجرای تست‌ها و بیلد — برای هر دو ایجنت
#
#  چرا این فایل وجود دارد:
#  Go در محیط توسعه‌ی Claude نصب نیست و شبکه بسته است، پس من
#  نمی‌توانم کد را اجرا کنم. هر ایجنتی که به ترمینال دسترسی دارد
#  باید از این اسکریپت استفاده کند تا نتیجه یکسان باشد.
#
#  اجرا:  .\run-tests.ps1
# ============================================================

$ErrorActionPreference = "Continue"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Backend = Join-Path $Root "backend"

Write-Host ""
Write-Host "═══════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  NexusAI - Build & Test" -ForegroundColor Cyan
Write-Host "═══════════════════════════════════════════════" -ForegroundColor Cyan

Set-Location $Backend

# --- 1. Go vet: catches compile errors and suspicious code ---
Write-Host ""
Write-Host "▸ [1/4] go vet ./..." -ForegroundColor Yellow
go vet ./... 2>&1 | ForEach-Object { Write-Host $_ }
if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ go vet passed" -ForegroundColor Green
} else {
    Write-Host "  ✗ go vet FAILED (exit $LASTEXITCODE)" -ForegroundColor Red
}

# --- 2. gofmt: catches formatting and parse errors ---
Write-Host ""
Write-Host "▸ [2/4] gofmt -l (unformatted files)" -ForegroundColor Yellow
$gofmtOut = gofmt -l . 2>&1
if ($gofmtOut) {
    $gofmtOut | ForEach-Object { Write-Host "  $_" -ForegroundColor Yellow }
    Write-Host "  (these files need gofmt -w)" -ForegroundColor DarkGray
} else {
    Write-Host "  ✓ all files formatted" -ForegroundColor Green
}

# --- 3. go build ---
Write-Host ""
Write-Host "▸ [3/4] go build" -ForegroundColor Yellow
go build ./... 2>&1 | ForEach-Object { Write-Host $_ }
if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ build passed" -ForegroundColor Green
} else {
    Write-Host "  ✗ build FAILED (exit $LASTEXITCODE)" -ForegroundColor Red
}

# --- 4. go test with verbose output ---
Write-Host ""
Write-Host "▸ [4/4] go test ./..." -ForegroundColor Yellow
$testOutput = go test ./... 2>&1
$testOutput | ForEach-Object { Write-Host $_ }

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "  ✓ ALL TESTS PASSED" -ForegroundColor Green

    $passCount = ($testOutput | Select-String -Pattern "^ok\s" ).Count
    $failCount = ($testOutput | Select-String -Pattern "^FAIL" ).Count
    Write-Host "  packages passed: $passCount, failed: $failCount" -ForegroundColor Green
} else {
    Write-Host ""
    Write-Host "  ✗ TESTS FAILED" -ForegroundColor Red
    Write-Host ""
    Write-Host "  ── FAILED TESTS ──" -ForegroundColor Red
    $testOutput | Select-String -Pattern "^\s*--- FAIL|^--- FAIL|FAIL\s" | ForEach-Object {
        Write-Host "  $_" -ForegroundColor Red
    }
}

# --- Summary ---
Write-Host ""
Write-Host "═══════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  نتیجه را در coordination/LIVE_BOARD.md بنویسید" -ForegroundColor Cyan
Write-Host "═══════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# Restore .env into this process so the developer can run the daemon after
if (Test-Path (Join-Path $Root ".env")) {
    Get-Content (Join-Path $Root ".env") | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        [Environment]::SetEnvironmentVariable(
            $line.Substring(0, $idx).Trim(),
            $line.Substring($idx + 1).Trim().Trim('"'),
            "Process")
    }
    Write-Host "  .env loaded into this session." -ForegroundColor DarkGray
    Write-Host ""
}