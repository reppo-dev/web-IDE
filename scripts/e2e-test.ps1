# ============================================================
#  تست end-to-end روی پروژه‌ی واقعی
#
#  چرا این فایل وجود دارد:
#  Claude به پوشه‌ی پروژه‌ها دسترسی ندارد و Go در محیطش نصب نیست،
#  پس این اسکریپت پل بین آن و سیستم واقعی است. هر کسی که ترمینال
#  ویندوز دارد می‌تواند آن را اجرا کند و گزارش بگیرد.
#
#  اجرا:
#    .\scripts\e2e-test.ps1 -ProjectPath G:\first_app -Prompt "یک اپلیکیشن ..."
#
#  این اسکریپت چه می‌کند:
#    ۱. سلامت بک‌اند را چک می‌کند
#    ۲. از طریق WebSocket یک اجرای واقعی شروع می‌کند
#    ۳. رویدادها را ضبط می‌کند و تحلیل می‌کند:
#       - آیا پلن ساخته شد؟
#       - آیا تسک‌ها موازی اجرا شدند؟ (تشخیص با چند task_id هم‌زمان)
#       - آیا هر تسک از دروازه‌ی verification رد شد؟
#       - آیا فایل قفل گرفت و آزاد شد؟
#       - آیا حلقه‌ی موازی گیر نکرد؟
#    ۴. خروجی فلاتر را بررسی می‌کند
#    ۵. گزارش می‌دهد
# ============================================================

param(
    [string]$ProjectPath = "G:\first_app",
    [string]$Prompt = "یک اپلیکیشن کپی دوالینگو برای یادگیری زبان انگلیسی برای فارسی‌زبانان بسازید",
    [string]$TeamId = "team-developers-ninerouter",
    [int]$BackendPort = 8080,
    [string]$DaemonUrl = "",
    [int]$TimeoutMinutes = 20,
    [switch]$SkipFlutterCheck,
    [switch]$RunFlutterAnalyze
)

$ErrorActionPreference = "Continue"

function Write-Section($Text) {
    Write-Host ""
    Write-Host "═══════════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host "  $Text" -ForegroundColor Cyan
    Write-Host "═══════════════════════════════════════════════" -ForegroundColor Cyan
}

function Write-Ok($Text)   { Write-Host "  ✓ $Text" -ForegroundColor Green }
function Write-Fail($Text) { Write-Host "  ✗ $Text" -ForegroundColor Red }
function Write-Info($Text) { Write-Host "  · $Text" -ForegroundColor DarkGray }
function Write-Warn2($Text){ Write-Host "  ! $Text" -ForegroundColor Yellow }

# ------------------------------------------------------------
# 1. سلامت بک‌اند
# ------------------------------------------------------------
Write-Section "۱. بررسی سلامت سرویس‌ها"

if ([string]::IsNullOrEmpty($DaemonUrl)) {
    $DaemonUrl = "ws://127.0.0.1:$BackendPort/ws"
}

$healthUrl = "http://127.0.0.1:$BackendPort/health"
try {
    $health = Invoke-RestMethod -Uri $healthUrl -TimeoutSec 5
    Write-Ok "بک‌اند سالم است: $($health | ConvertTo-Json -Compress)"
} catch {
    Write-Fail "بک‌اند در دسترس نیست ($healthUrl)"
    Write-Host ""
    Write-Host "  قبل از ادامه، بک‌اند را اجرا کنید:" -ForegroundColor Yellow
    Write-Host "    cd backend; go build -o nexusd.exe cmd/nexusd; .\nexusd.exe" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  یا اسکریپت تست کامل را اجرا کنید: .\run-tests.ps1" -ForegroundColor Gray
    exit 1
}

# ------------------------------------------------------------
# 2. بررسی مسیر پروژه
# ------------------------------------------------------------
Write-Section "۲. بررسی پروژه‌ی مقصد"

if (-not (Test-Path $ProjectPath)) {
    Write-Fail "مسیر پروژه پیدا نشد: $ProjectPath"
    Write-Host "  با پارامتر -ProjectPath مسیر درست را بدهید." -ForegroundColor Gray
    exit 1
}
Write-Ok "پروژه پیدا شد: $ProjectPath"

$isFlutter = Test-Path (Join-Path $ProjectPath "pubspec.yaml")
if ($isFlutter) {
    Write-Ok "پروژه Flutter است (pubspec.yaml موجود)"
} else {
    Write-Warn2 "pubspec.yaml پیدا نشد — این پروژه Flutter به نظر نمی‌رسد"
    if (-not $SkipFlutterCheck) {
        $answer = Read-Host "  با این حال ادامه می‌دهی؟ (y/n)"
        if ($answer -ne "y") { exit 1 }
    }
}

# شمارش فایل‌های فعلی تا بعدا بتوانیم تشخیص دهیم چیزی اضافه شده
function Get-SourceFileCount($Path) {
    if (-not (Test-Path $Path)) { return 0 }
    return (Get-ChildItem -Path $Path -Recurse -File `
        -Include "*.dart", "*.ts", "*.tsx", "*.js", "*.go", "*.py" `
        -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -notmatch '\\(build|\.dart_tool|node_modules|\.git)\\' } |
        Measure-Object).Count
}
$filesBefore = Get-SourceFileCount $ProjectPath
Write-Info "فایل‌های فعلی پروژه: $filesBefore"

# ------------------------------------------------------------
# 3. شروع اجرای واقعی از طریق WebSocket
# ------------------------------------------------------------
Write-Section "۳. شروع اجرای swarm (این مرحله طول می‌کشد)"

# WebSocket در PowerShell 5.1 نیاز به .NET ClientWebSocket دارد.
# برای اینکه اسکریپت روی هر دو نسخه کار کند، از System.Net.WebSockets
# استفاده می‌کنیم و در صورت شکست، مسیر جایگزین را پیشنهاد می‌دهیم.

$script:events = New-Object System.Collections.ArrayList
$script:planTasks = @{}
$script:taskStreams = @{}
$script:lockEvents = New-Object System.Collections.ArrayList
$script:roundStarts = @()
$script:consensusText = ""
$script:errors = New-Object System.Collections.ArrayList
$script:verdictPasses = @()
$script:tokenTotal = 0
$script:parallelEvidence = @{}

Add-Type -AssemblyName System.Net.WebSockets 2>$null
Add-Type -AssemblyName System.Net.Http 2>$null

$ws = $null
$connected = $false

try {
    $ws = New-Object System.Net.WebSockets.ClientWebSocket
    $cts = New-Object System.Threading.CancellationTokenSource
    $cts.CancelAfter(10000)
    Write-Info "در حال اتصال به $DaemonUrl ..."
    $ws.ConnectAsync([Uri]$DaemonUrl, $cts.Token).GetAwaiter().GetResult()
    $connected = $true
    Write-Ok "متصل شدیم"
} catch {
    Write-Fail "اتصال وب‌سوکت ناموفق بود: $($_.Exception.Message)"
    Write-Host ""
    Write-Host "  اگر بک‌اند روی 127.0.0.1:8080 نیست، -BackendPort را بدهید." -ForegroundColor Gray
    Write-Host "  اگر WSS لازم دارید، -DaemonUrl را کامل بدهید." -ForegroundColor Gray
    exit 1
}

# استعلام شناسه واقعی پروژه از رجیستری بک‌اند
$projectId = ""
try {
    $projs = Invoke-RestMethod -Uri "http://127.0.0.1:$BackendPort/api/projects" -TimeoutSec 5
    foreach ($p in $projs) {
        if ($p.root.TrimEnd('\') -eq $ProjectPath.TrimEnd('\')) {
            $projectId = $p.id
            break
        }
    }
} catch { }

if ([string]::IsNullOrEmpty($projectId)) {
    try {
        $regBody = @{ path = $ProjectPath } | ConvertTo-Json
        $pRef = Invoke-RestMethod -Uri "http://127.0.0.1:$BackendPort/api/projects" -Method Post -Body $regBody -ContentType "application/json" -TimeoutSec 5
        $projectId = $pRef.id
    } catch {
        $projectId = [System.IO.Path]::GetFileName($ProjectPath.TrimEnd('\'))
    }
}

Write-Info "project_id: $projectId"
Write-Info "prompt: $Prompt"
Write-Info "team_id: $TeamId"

$startPayload = @{
    mode = "rolebased"
}
if (-not [string]::IsNullOrEmpty($TeamId)) {
    $startPayload["team_id"] = $TeamId
}

$startMsg = @{
    action      = "start_swarm"
    project_id  = $projectId
    user_prompt = $Prompt
    payload     = $startPayload
} | ConvertTo-Json -Depth 6 -Compress

$bytes = [System.Text.Encoding]::UTF8.GetBytes($startMsg)
$seg = New-Object System.ArraySegment[byte] -ArgumentList @(,$bytes)
try {
    $ws.SendAsync($seg, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, [System.Threading.CancellationToken]::None).GetAwaiter().GetResult()
    Write-Ok "درخواست اجرا ارسال شد"
} catch {
    Write-Fail "ارسال درخواست ناموفق بود: $($_.Exception.Message)"
    exit 1
}

# ------------------------------------------------------------
# 4. ضبط رویدادها
# ------------------------------------------------------------
Write-Section "۴. ضبط رویدادها (تا $TimeoutMinutes دقیقه)"

$buffer = New-Object byte[] 65536
$deadline = (Get-Date).AddMinutes($TimeoutMinutes)
$lastActivity = Get-Date
$eventCount = 0

function Handle-Event($evt) {
    $script:eventCount++
    $type = $evt.type
    $script:lastActivity = Get-Date

    switch ($type) {
        "plan_updated" {
            if ($evt.plan -and $evt.plan.tasks) {
                foreach ($t in $evt.plan.tasks) {
                    if (-not $script:planTasks.ContainsKey($t.id)) {
                        $script:planTasks[$t.id] = $t.state
                    } else {
                        $script:planTasks[$t.id] = $t.state
                    }
                }
                $done = ($evt.plan.tasks | Where-Object { $_.state -eq "DONE" }).Count
                $total = $evt.plan.tasks.Count
                Write-Host ("  [{0,4}] پلن: {1}/{2} تسک تمام شد ({3}٪)" -f $script:eventCount, $done, $total, [int](100*$done/[Math]::Max(1,$total))) -ForegroundColor DarkCyan
            }
        }

        "task_step" {
            $taskId = $evt.task_id
            $who = if ($evt.stream_agent) { $evt.stream_agent } else { $evt.agent }
            if (-not $script:taskStreams.ContainsKey($taskId)) {
                $script:taskStreams[$taskId] = New-Object System.Collections.ArrayList
            }
            [void]$script:taskStreams[$taskId].Add(@{
                agent = $who
                kind  = $evt.stream.kind
                text  = $evt.stream.text
                at    = $evt.stream.at
            })
            # شواهد موازی‌سازی: دو تسک که در یک ثانیه گام برداشتند
            $sec = [int][double]::Parse((Get-Date -UFormat %s))
            if (-not $script:parallelEvidence.ContainsKey($sec)) {
                $script:parallelEvidence[$sec] = New-Object System.Collections.ArrayList
            }
            [void]$script:parallelEvidence[$sec].Add($taskId)
        }

        "locks_changed" {
            if ($evt.locks) {
                [void]$script:lockEvents.Add(@{ round = $evt.round; locks = $evt.locks })
            }
        }

        "round_started" {
            $script:roundStarts += $evt.round
            Write-Host ("  [{0,4}] دور {1} شروع شد" -f $script:eventCount, $evt.round) -ForegroundColor DarkGray
        }

        "agent_output" {
            # نتیجه‌ی تست را از متن پیام بیرون می‌کشیم
            $text = if ($evt.payload -is [string]) { $evt.payload } else { "" }
            if ($text -match "تست‌ها پاس شدند")   { $script:verdictPasses += "pass" }
            if ($text -match "تست‌ها پاس نشدند")  { $script:verdictPasses += "fail" }
            if ($text.Length -gt 0) {
                Write-Host ("  [{0,4}] {1}: {2}" -f $script:eventCount, $evt.agent, $text) -ForegroundColor DarkGray
            }
        }

        "consensus_reached" {
            $script:consensusText = if ($evt.payload -is [string]) { $evt.payload } else { "" }
            Write-Host ("  [{0,4}] اجرا پایان یافت" -f $script:eventCount) -ForegroundColor Green
        }

        "error" {
            [void]$script:errors.Add($evt.payload)
            Write-Host ("  [{0,4}] خطا: {1}" -f $script:eventCount, $evt.payload) -ForegroundColor Red
        }
    }

    # توکن مصرفی جمع می‌شود تا گزارش بدهد اجرا چقدر هزینه داشته. نبودنش خطا
    # نیست؛ فقط یعنی رویداد حاملش نبوده.
    if ($evt.usage) {
        foreach ($k in $evt.usage.PSObject.Properties) {
            $script:tokenTotal += [int]$k.Value.total_tokens
        }
    }
}

Write-Host "  در حال دریافت رویدادها... (Ctrl+C برای توقف)" -ForegroundColor DarkGray

$globalCts = New-Object System.Threading.CancellationTokenSource
$globalCts.CancelAfter([int]([Math]::Max(1000, ($deadline - (Get-Date)).TotalMilliseconds)))

while ((Get-Date) -lt $deadline -and $ws.State -eq [System.Net.WebSockets.WebSocketState]::Open) {
    try {
        $ms = New-Object System.IO.MemoryStream
        do {
            $seg = New-Object System.ArraySegment[byte] -ArgumentList @(,$buffer)
            $res = $ws.ReceiveAsync($seg, $globalCts.Token).GetAwaiter().GetResult()
            if ($res.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
                Write-Info "اتصال وب‌سوکت توسط سرور بسته شد."
                break
            }
            if ($res.Count -gt 0) { $ms.Write($buffer, 0, $res.Count) }
        } while (-not $res.EndOfMessage)

        if ($ms.Length -gt 0) {
            $text = [System.Text.Encoding]::UTF8.GetString($ms.ToArray())
            try {
                $evt = $text | ConvertFrom-Json
                Handle-Event $evt
            } catch {
                Write-Warn2 "پیام قابل تفسیر نبود ($($_.Exception.Message)): $($text.Substring(0, [Math]::Min(120, $text.Length)))"
            }
        }

        # اگر ۱۲۰ ثانیه هیچ رویدادی نیامد، یعنی احتمالاً گیر کرده
        if (((Get-Date) - $script:lastActivity).TotalSeconds -gt 120) {
            Write-Warn2 "۱۲۰ ثانیه هیچ رویدادی نرسید — احتمالاً اجرا متوقف شده"
            break
        }
    } catch [System.OperationCanceledException] {
        Write-Warn2 "زمان مهلت اجرا به پایان رسید"
        break
    } catch {
        if ($_.Exception.Message -match "closed|aborted") {
            Write-Warn2 "اتصال بسته شد: $($_.Exception.Message)"
            break
        }
        Write-Warn2 "خطای دریافت: $($_.Exception.Message)"
        break
    }

    if ($script:consensusText -ne "") { break }
}

try { $ws.Dispose() } catch { }

# ------------------------------------------------------------
# 5. تحلیل نتایج
# ------------------------------------------------------------
Write-Section "۵. تحلیل نتایج"

$failures = @()

# ۵.۱ آیا پلن ساخته شد؟
if ($script:planTasks.Count -eq 0) {
    Write-Fail "هیچ پلنی ساخته نشد"
    $failures += "پلن ساخته نشد"
} else {
    $totalTasks = $script:planTasks.Count
    $doneTasks = ($script:planTasks.Values | Where-Object { $_ -eq "DONE" }).Count
    Write-Ok "پلن ساخته شد: $totalTasks تسک، $doneTasks تمام شده"
    if ($doneTasks -lt $totalTasks) {
        Write-Warn2 "همه‌ی تسک‌ها تمام نشدند ($doneTasks از $totalTasks)"
    }
}

# ۵.۲ آیا موازی‌سازی واقعاً رخ داد؟
$parallelSeconds = 0
$maxConcurrent = 0
foreach ($sec in $script:parallelEvidence.Keys) {
    $distinct = ($script:parallelEvidence[$sec] | Select-Object -Unique).Count
    if ($distinct -gt $maxConcurrent) { $maxConcurrent = $distinct }
    if ($distinct -gt 1) { $parallelSeconds++ }
}
if ($maxConcurrent -gt 1) {
    Write-Ok "موازی‌سازی تأیید شد: بیشترین تعداد تسک هم‌زمان = $maxConcurrent"
} else {
    Write-Warn2 "هیچ دو تسکی هم‌زمان اجرا نشدند (اجرا ترتیبی بود یا تسک‌ها کم بودند)"
}

# ۵.۳ آیا قفل فایل استفاده شد؟
if ($script:lockEvents.Count -gt 0) {
    Write-Ok "رویداد قفل فایل دریافت شد: $($script:lockEvents.Count) بار"
} else {
    Write-Info "رویداد قفل فایل دریافت نشد (ممکن است هیچ برخوردی رخ نداده باشد)"
}

# ۵.۴ آیا تسک‌ها از verification رد شدند؟
$passCount = ($script:verdictPasses | Where-Object { $_ -eq "pass" }).Count
$failCount = ($script:verdictPasses | Where-Object { $_ -eq "fail" }).Count
Write-Info "دروازه‌ی verification: $passCount قبول، $failCount رد"
if ($passCount -eq 0 -and $failCount -eq 0) {
    Write-Warn2 "هیچ نتیجه‌ی verification دریافت نشد"
}

# ۵.۵ آیا اجماع پایان یافت؟
if ($script:consensusText -ne "") {
    Write-Ok "پایان اجرا: $($script:consensusText)"
} else {
    Write-Warn2 "اجرا با پیام پایان نرسید (تایم‌اوت یا قطع اتصال)"
}

# ۵.۶ خطاها
if ($script:errors.Count -gt 0) {
    Write-Host ""
    Write-Host "  خطاهای گزارش‌شده:" -ForegroundColor Red
    foreach ($e in $script:errors) { Write-Host "    - $e" -ForegroundColor Red }
}

# ------------------------------------------------------------
# 6. بررسی خروجی پروژه
# ------------------------------------------------------------
Write-Section "۶. بررسی خروجی پروژه"

$filesAfter = Get-SourceFileCount $ProjectPath
$newFiles = $filesAfter - $filesBefore
Write-Info "فایل‌های قبل: $filesBefore  →  بعد: $filesAfter  (تغییر: $newFiles)"

if ($newFiles -gt 0) {
    Write-Ok "سیستم $newFiles فایل جدید تولید کرد"
} else {
    Write-Warn2 "هیچ فایل جدیدی تولید نشد"
}

if (-not $SkipFlutterCheck -and $isFlutter) {
    Write-Host ""
    Write-Info "در حال بررسی فایل‌های پروژه..."
    $dartFiles = Get-ChildItem -Path $ProjectPath -Recurse -File -Filter "*.dart" `
        -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -notmatch '\\(build|\.dart_tool)\\' }
    Write-Info "فایل‌های Dart: $($dartFiles.Count)"

    # آیا کد فلاتر واقعاً نوشته شده یا فقط فایل خالی ساخته شده؟
    $emptyDart = @()
    $tinyDart = @()
    foreach ($f in $dartFiles) {
        $len = (Get-Item $f.FullName).Length
        if ($len -eq 0) { $emptyDart += $f.Name }
        elseif ($len -lt 60) { $tinyDart += "$($f.Name) ($len بایت)" }
    }

    if ($emptyDart.Count -gt 0) {
        Write-Host ""
        Write-Host "  فایل‌های خالی: $($emptyDart -join ', ')" -ForegroundColor Red
        $failures += "فایل خالی ساخته شده"
    }
    if ($tinyDart.Count -gt 0) {
        Write-Host ""
        Write-Host "  فایل‌های بسیار کوچک (احتمالاً ناقص):" -ForegroundColor Yellow
        $tinyDart | ForEach-Object { Write-Host "    - $_" -ForegroundColor Yellow }
    }

    if ($RunFlutterAnalyze) {
        Write-Host ""
        Write-Info "در حال اجرای flutter analyze..."
        Push-Location $ProjectPath
        try {
            $analyzeOut = & flutter analyze 2>&1 | Out-String
            if ($LASTEXITCODE -eq 0) {
                Write-Ok "flutter analyze تمیز است"
            } else {
                Write-Host "  خروجی analyze:" -ForegroundColor Yellow
                Write-Host $analyzeOut -ForegroundColor DarkYellow
                $failures += "flutter analyze خطا دارد"
            }
        } catch {
            Write-Warn2 "flutter در دسترس نیست: $($_.Exception.Message)"
        } finally {
            Pop-Location
        }

        Write-Info "در حال اجرای flutter test..."
        Push-Location $ProjectPath
        try {
            $testOut = & flutter test 2>&1 | Out-String
            if ($LASTEXITCODE -eq 0) {
                Write-Ok "flutter test پاس شد"
            } else {
                Write-Host $testOut -ForegroundColor DarkYellow
                $failures += "flutter test پاس نشد"
            }
        } catch {
            Write-Warn2 "flutter test اجرا نشد: $($_.Exception.Message)"
        } finally {
            Pop-Location
        }
    } else {
        Write-Info "برای بررسی کامپایل، با -RunFlutterAnalyze اجرا کنید"
    }
}

# ------------------------------------------------------------
# 7. گزارش نهایی
# ------------------------------------------------------------
Write-Section "۷. گزارش نهایی"

$totalEvents = $script:eventCount
$totalTokens = $script:tokenTotal

Write-Host ""
Write-Host "  رویداد دریافتی      : $totalEvents"
Write-Host "  تسک‌های پلن         : $($script:planTasks.Count)"
Write-Host "  تسک‌های جریان‌دار    : $($script:taskStreams.Count)"
Write-Host "  بیشترین اجرای موازی  : $maxConcurrent ثانیه"
Write-Host "  رویداد قفل          : $($script:lockEvents.Count)"
Write-Host "  دورها               : $($script:roundStarts.Count)"
Write-Host "  مصرف توکن           : $totalTokens"
Write-Host "  خطا                 : $($script:errors.Count)"
Write-Host ""

if ($failures.Count -eq 0) {
    Write-Host "  ══════════════════════════════════════════════" -ForegroundColor Green
    Write-Host "   نتیجه: تست با موفقیت گذشت" -ForegroundColor Green
    Write-Host "  ══════════════════════════════════════════════" -ForegroundColor Green
    exit 0
} else {
    Write-Host "  ══════════════════════════════════════════════" -ForegroundColor Red
    Write-Host "   نتیجه: $($failures.Count) مشکل پیدا شد" -ForegroundColor Red
    Write-Host "  ══════════════════════════════════════════════" -ForegroundColor Red
    foreach ($f in $failures) { Write-Host "     ✗ $f" -ForegroundColor Red }
    exit 1
}