$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskState = Join-Path $taskRoot '.local\processes.json'
if (-not (Test-Path -LiteralPath $taskState)) { Write-Output 'No managed local processes.'; exit 0 }
$taskProcesses = Get-Content -LiteralPath $taskState -Raw | ConvertFrom-Json
foreach ($taskName in @('mailbox','admin','api')) {
    if (-not $taskProcesses.$taskName) { continue }
    $taskProcess = Get-Process -Id $taskProcesses.$taskName -ErrorAction SilentlyContinue
    $taskExpected = Join-Path $taskRoot "backend\bin\$taskName.exe"
    $taskExecutable = if ($taskProcess) { (Get-CimInstance Win32_Process -Filter "ProcessId = $($taskProcess.Id)").ExecutablePath }
    if ($taskProcess -and $taskExecutable -eq $taskExpected) {
        Stop-Process -Id $taskProcess.Id
        $taskProcess.WaitForExit()
        Write-Output "$taskName stopped."
    }
}
Remove-Item -LiteralPath $taskState
