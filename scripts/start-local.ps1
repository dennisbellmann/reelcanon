param([switch]$Rebuild)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskBackend = Join-Path $taskRoot 'backend'
$taskLocal = Join-Path $taskRoot '.local'
New-Item -ItemType Directory -Force -Path $taskLocal | Out-Null
if (-not (Test-Path -LiteralPath (Join-Path $taskRoot '.env'))) { throw 'Copy .env.example to .env and configure the database and admin password first.' }

function Get-LocalOrigin([string]$Name, [string]$Default) {
    $taskAddress = [Environment]::GetEnvironmentVariable($Name)
    if (-not $taskAddress) {
        $taskLine = Get-Content -LiteralPath (Join-Path $taskRoot '.env') | Where-Object { $_ -match "^\s*$Name\s*=" } | Select-Object -Last 1
        if ($taskLine) { $taskAddress = ($taskLine -split '=', 2)[1].Trim().Trim('"').Trim("'") }
    }
    if (-not $taskAddress) { $taskAddress = $Default }
    $taskAddress = $taskAddress -replace '^:', '127.0.0.1:' -replace '^0\.0\.0\.0:', '127.0.0.1:' -replace '^\[::\]:', '[::1]:'
    return "http://$taskAddress"
}

function Get-ClientWebsiteOrigin {
    $taskAddressLine = Get-Content -LiteralPath (Join-Path $taskRoot '.env') | Where-Object { $_ -match '^\s*API_ADDR\s*=' } | Select-Object -Last 1
    $taskAddress = if ($taskAddressLine) { ($taskAddressLine -split '=', 2)[1].Trim().Trim('"').Trim("'") } else { '' }
    if ($taskAddress -match '^(?::|0\.0\.0\.0:|\[::\]:)(\d+)$') {
        $taskNetwork = Get-NetIPConfiguration | Where-Object { $_.IPv4DefaultGateway -and $_.NetAdapter.Status -eq 'Up' } | Select-Object -First 1
        $taskIPv4 = $taskNetwork.IPv4Address | Where-Object { $_.IPAddress -notlike '169.254.*' } | Select-Object -First 1 -ExpandProperty IPAddress
        if (-not $taskIPv4) { throw 'No private IPv4 address found. Connect this PC to your home network.' }
        return "http://${taskIPv4}:$($Matches[1])"
    }
    return $taskWebsite
}

function Wait-LocalService($Process, [string]$Url, [int]$Status) {
    $taskDeadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $taskDeadline) {
        $Process.Refresh()
        if ($Process.HasExited) { throw "Local service exited before becoming ready. See logs in $taskLocal." }
        try {
            $taskResponse = $taskClient.GetAsync($Url).GetAwaiter().GetResult()
            try {
                if ([int]$taskResponse.StatusCode -eq $Status) { return }
            } finally { $taskResponse.Dispose() }
        } catch { }
        Start-Sleep -Milliseconds 300
    }
    throw "Local service did not become ready at $Url. See logs in $taskLocal."
}

function Confirm-LocalServices([object[]]$Processes) {
    Add-Type -AssemblyName System.Net.Http
    $taskHandler = New-Object System.Net.Http.HttpClientHandler
    $taskHandler.UseProxy = $false
    $taskClient = New-Object System.Net.Http.HttpClient($taskHandler)
    $taskClient.Timeout = [TimeSpan]::FromSeconds(2)
    try {
        Write-Output 'Checking website, API, cockpit and development mail...'
        Wait-LocalService $Processes[0] "$taskWebsite/api/v1/healthz" 200
        Wait-LocalService $Processes[0] "$taskWebsite/canons" 200
        # The protected cockpit is ready when it requests Basic Authentication.
        Wait-LocalService $Processes[1] "$taskCockpit/canons" 401
        Wait-LocalService $Processes[2] 'http://127.0.0.1:8025' 200
        foreach ($taskProcess in $Processes) {
            $taskProcess.Refresh()
            if ($taskProcess.HasExited) { throw "Local service exited. See logs in $taskLocal." }
        }
        Write-Output ''
        Write-Output 'Reel Canon is ready.'
        Write-Output "Website: $taskClientWebsite/canons"
        if ($taskClientWebsite -ne $taskWebsite) { Write-Output "API: $taskClientWebsite/api/v1" }
        Write-Output "Cockpit: $taskCockpit/canons (credentials from .env)"
        Write-Output 'Development mail: http://127.0.0.1:8025'
        Write-Output 'Stop: make stop | Restart and rebuild: make restart'
    } finally {
        $taskClient.Dispose()
    }
}

$taskWebsite = Get-LocalOrigin 'API_ADDR' ':8080'
$taskClientWebsite = Get-ClientWebsiteOrigin
$taskCockpit = Get-LocalOrigin 'ADMIN_ADDR' ':8081'
$taskPortable = Join-Path $env:LOCALAPPDATA 'ReelCanon\postgres'
$taskController = Join-Path $taskPortable 'pgsql\bin\pg_ctl.exe'
if (Test-Path -LiteralPath $taskController) {
    & $taskController -D (Join-Path $taskPortable 'data') status *> $null
    if ($LASTEXITCODE -ne 0) {
        & $taskController -D (Join-Path $taskPortable 'data') -l (Join-Path $taskPortable 'postgres.log') -o '-h 127.0.0.1 -p 54329' -w start
        if ($LASTEXITCODE -ne 0) { throw 'Portable PostgreSQL failed to start.' }
    }
}
$taskState = Join-Path $taskLocal 'processes.json'
$taskRunning = @()
if (Test-Path -LiteralPath $taskState) {
    $taskExisting = Get-Content -LiteralPath $taskState -Raw | ConvertFrom-Json
    foreach ($taskName in @('api','admin','mailbox')) {
        if (-not $taskExisting.$taskName) { continue }
        $taskProcess = Get-Process -Id $taskExisting.$taskName -ErrorAction SilentlyContinue
        # A 32-bit Make can launch 32-bit PowerShell, where Process.Path is empty for 64-bit Go binaries.
        $taskExecutable = if ($taskProcess) { (Get-CimInstance Win32_Process -Filter "ProcessId = $($taskProcess.Id)").ExecutablePath }
        if ($taskProcess -and $taskExecutable -eq (Join-Path $taskBackend "bin\$taskName.exe")) { $taskRunning += $taskProcess }
    }
}
if ($taskRunning.Count -eq 3) {
    Write-Output 'Reel Canon is already running.'
    Confirm-LocalServices $taskRunning
    return
}
if ($taskRunning.Count -gt 0) { throw 'Only some Reel Canon services are running. Run make restart to start all services.' }
Push-Location $taskRoot
$taskProcesses = @{}
$taskStarted = @()
$taskOldFrontend = $env:FRONTEND_DIST
try {
    if ($Rebuild -or -not (Test-Path -LiteralPath (Join-Path $taskBackend 'bin\api.exe')) -or -not (Test-Path -LiteralPath (Join-Path $taskBackend 'bin\admin.exe')) -or -not (Test-Path -LiteralPath (Join-Path $taskBackend 'bin\mailbox.exe')) -or -not (Test-Path -LiteralPath (Join-Path $taskRoot 'frontend\dist\index.html'))) {
        & node scripts/build-backend.mjs
        if ($LASTEXITCODE -ne 0) { throw 'Backend build failed.' }
        & npm --prefix frontend run build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    }
    & go -C backend run ./cmd/migrate -dir ../database/migrations
    if ($LASTEXITCODE -ne 0) { throw 'Database migration failed.' }
    $env:FRONTEND_DIST = Join-Path $taskRoot 'frontend\dist'
    foreach ($taskName in @('api','admin','mailbox')) {
        $taskProcess = Start-Process -FilePath (Join-Path $taskBackend "bin\$taskName.exe") -WorkingDirectory $taskBackend -WindowStyle Hidden -RedirectStandardOutput (Join-Path $taskLocal "$taskName.log") -RedirectStandardError (Join-Path $taskLocal "$taskName-error.log") -PassThru
        $taskProcesses[$taskName] = $taskProcess.Id
        $taskStarted += $taskProcess
    }
    $taskProcesses | ConvertTo-Json | Set-Content -LiteralPath $taskState
    Confirm-LocalServices $taskStarted
} catch {
    foreach ($taskProcess in $taskStarted) {
        $taskProcess.Refresh()
        if (-not $taskProcess.HasExited) { Stop-Process -Id $taskProcess.Id -ErrorAction SilentlyContinue }
    }
    if ($taskStarted.Count -gt 0 -and (Test-Path -LiteralPath $taskState)) { Remove-Item -LiteralPath $taskState }
    throw
} finally {
    $env:FRONTEND_DIST = $taskOldFrontend
    Pop-Location
}
