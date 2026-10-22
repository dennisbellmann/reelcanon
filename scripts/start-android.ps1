param(
    [string]$Avd = 'Pixel_8a_API_35',
    [string]$Serial = 'emulator-5554',
    [switch]$SkipBuild,
    [switch]$Test
)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskSdk = $env:ANDROID_HOME
if (-not $taskSdk) { $taskSdk = Join-Path $env:LOCALAPPDATA 'Android\Sdk' }
$taskAdb = Join-Path $taskSdk 'platform-tools\adb.exe'
$taskEmulator = Join-Path $taskSdk 'emulator\emulator.exe'
if (-not (Test-Path -LiteralPath $taskAdb)) { throw 'Android SDK not found. Set ANDROID_HOME.' }

# Prefer a usable existing JAVA_HOME; the older Android Studio path on this machine is incomplete.
if (-not $env:JAVA_HOME -or -not (Test-Path -LiteralPath (Join-Path $env:JAVA_HOME 'lib\jvm.cfg'))) {
    $taskJava = 'C:\Program Files\Android\Android Studio Q\jbr'
    if (Test-Path -LiteralPath (Join-Path $taskJava 'lib\jvm.cfg')) { $env:JAVA_HOME = $taskJava }
    else { throw 'Set JAVA_HOME to a working JDK 17 or newer.' }
}
$env:ANDROID_HOME = $taskSdk
$taskDevices = & $taskAdb devices
if (-not ($taskDevices -match "^$([regex]::Escape($Serial))\s+device$")) {
    if (-not (Test-Path -LiteralPath $taskEmulator)) { throw 'Android emulator executable not found.' }
    $taskAvailable = & $taskEmulator -list-avds
    if ($Avd -notin $taskAvailable) { throw "AVD '$Avd' is not installed. Available: $($taskAvailable -join ', ')" }
    if ($Serial -notmatch '^emulator-(\d+)$') { throw 'Use an emulator serial, e.g. emulator-5554.' }
    $taskPort = $Matches[1]
    $taskLocal = Join-Path $taskRoot '.local'
    New-Item -ItemType Directory -Force -Path $taskLocal | Out-Null
    Start-Process -FilePath $taskEmulator -ArgumentList @('-avd', $Avd, '-port', $taskPort, '-no-snapshot-load', '-no-boot-anim') -WindowStyle Hidden -RedirectStandardOutput (Join-Path $taskLocal 'mobile-emulator.log') -RedirectStandardError (Join-Path $taskLocal 'mobile-emulator-error.log') | Out-Null
}
$taskDeadline = (Get-Date).AddMinutes(3)
do {
    $taskBoot = & $taskAdb -s $Serial shell getprop sys.boot_completed 2>$null
    if ($taskBoot -eq '1') { break }
    if ((Get-Date) -gt $taskDeadline) { throw 'Emulator did not finish booting within three minutes.' }
    Start-Sleep -Seconds 2
} while ($true)

try {
    $taskCatalog = Invoke-RestMethod 'http://127.0.0.1:8080/api/v1/canons' -TimeoutSec 5
    if ($taskCatalog.Count -eq 0) { Write-Warning 'The API has no published Canons yet.' }
} catch { throw 'Start the existing Go catalog first: ./scripts/start-local.ps1' }

Push-Location (Join-Path $taskRoot 'mobile')
try {
    if (-not $SkipBuild) {
        & ./gradlew.bat :androidApp:assembleDebug
        if ($LASTEXITCODE -ne 0) { throw 'Android build failed.' }
    }
    if ($Test) {
        $env:ANDROID_SERIAL = $Serial
        & ./gradlew.bat :composeApp:testAndroidHostTest :androidApp:lintDebug :androidApp:connectedDebugAndroidTest '-Pandroid.testInstrumentationRunnerArguments.liveCatalog=true'
        if ($LASTEXITCODE -ne 0) { throw 'Mobile checks failed.' }
    }
    & $taskAdb -s $Serial install -r 'androidApp/build/outputs/apk/debug/androidApp-debug.apk'
    if ($LASTEXITCODE -ne 0) { throw 'APK installation failed.' }
    & $taskAdb -s $Serial shell am start -W -n 'com.reelcanon.app/.MainActivity'
    if ($LASTEXITCODE -ne 0) { throw 'App launch failed.' }
    Write-Output "Reel Canon runs on $Serial ($Avd)."
} finally { Pop-Location }
