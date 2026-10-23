param(
    [ValidateSet('amd64','arm64')][string]$Arch,
    [Parameter(Mandatory)][string]$TokenProbe
)
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') {
    throw 'Non-admin acceptance requires a disposable hosted Windows runner.'
}
$SideletRoot = Split-Path $PSScriptRoot -Parent
Set-Location $SideletRoot
$Results = Join-Path $SideletRoot "build/results/windows-ci-$Arch/nonadmin"
New-Item -ItemType Directory -Force $Results | Out-Null
& $TokenProbe check $PID (Join-Path $Results 'powershell-token.json')
if ($LASTEXITCODE -ne 0) { throw 'Lifecycle PowerShell process is not running with the required reduced permissions.' }

# A behavioral negative check as well as inspecting token metadata. Request a
# write-capable handle to an existing system key, without creating/changing it.
$Machine = [Microsoft.Win32.RegistryKey]::OpenBaseKey('LocalMachine','Registry64')
$Denied = $false
try {
    try {
        $Key = $Machine.OpenSubKey('SOFTWARE',$true)
        if ($null -ne $Key) { $Key.Dispose() }
    } catch {
        if ($_.Exception -is [UnauthorizedAccessException] -or $_.Exception.InnerException -is [UnauthorizedAccessException]) {
            $Denied = $true
        } else { throw }
    }
} finally { $Machine.Dispose() }
if (-not $Denied) { throw 'Reduced process unexpectedly has machine-wide software registry write access.' }
@{ machineSoftwareWriteAccess = 'denied'; modifiedSystemRegistry = $false } |
    ConvertTo-Json | Set-Content (Join-Path $Results 'access-check.json') -Encoding utf8

$Info = Get-Content -Raw 'packaging/macos/app.json' | ConvertFrom-Json
$Name = "Sidelet-$($Info.version)-build$($Info.build)-windows-$Arch-local"
$env:SIDELET_SMOKE_EXE = Join-Path $SideletRoot "build/results/windows-ci-$Arch/unpacked/$Name/Sidelet.exe"
$env:SIDELET_SMOKE_OUTPUT = Join-Path $Results 'smoke'
try {
    # Compile while elevated, execute both the test and the actual packaged EXE
    # here. SQLite fixture creation, migration, writes and reopen all run reduced.
    & (Join-Path $SideletRoot "build/bin/sidelet-smoke-$Arch.test.exe") `
        -test.v -test.timeout=2m '-test.run=^TestPackagedApplication(Smoke|Upgrade)$' 2>&1 |
        Tee-Object -FilePath (Join-Path $Results 'packaged-tests.log')
    if ($LASTEXITCODE -ne 0) { throw 'Non-admin packaged smoke/upgrade tests failed.' }
    & (Join-Path $PSScriptRoot 'test-windows-installer-lifecycle.ps1') -Arch $Arch -TokenProbe $TokenProbe -SmokeRoot $env:SIDELET_SMOKE_OUTPUT
} finally {
    Remove-Item Env:SIDELET_SMOKE_EXE -ErrorAction SilentlyContinue
    Remove-Item Env:SIDELET_SMOKE_OUTPUT -ErrorAction SilentlyContinue
}
@{ passed = $true; scope = 'restricted medium-integrity token in the hosted runner account; not a separate standard-user logon'; arch = $Arch } |
    ConvertTo-Json | Set-Content (Join-Path $Results 'result.json') -Encoding utf8
