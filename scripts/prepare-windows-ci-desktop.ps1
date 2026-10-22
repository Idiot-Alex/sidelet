# Only the disposable GitHub-hosted ARM64 image. Never use on a personal or
# self-hosted machine. The image can start with OOBE above even topmost windows:
# https://github.com/actions/runner-images/issues/14069
$ErrorActionPreference = 'Stop'
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or
    $env:RUNNER_ENVIRONMENT -ne 'github-hosted' -or $env:RUNNER_ARCH -ne 'ARM64') {
    throw 'Desktop preparation is restricted to disposable GitHub-hosted Windows ARM64 runners.'
}

$OobePolicy = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\OOBE'
New-Item -Path $OobePolicy -Force | Out-Null
Set-ItemProperty -Path $OobePolicy -Name DisablePrivacyExperience -Type DWord -Value 1
$Engagement = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\UserProfileEngagement'
New-Item -Path $Engagement -Force | Out-Null
Set-ItemProperty -Path $Engagement -Name ScoobeSystemSettingEnabled -Type DWord -Value 0
$TestSession = (Get-Process -Id $PID).SessionId
foreach ($Name in 'UserOOBEBroker','WWAHost','SystemPropertiesPerformance') {
    Get-Process -Name $Name -ErrorAction SilentlyContinue |
        Where-Object { $_.SessionId -eq $TestSession } | ForEach-Object {
            Write-Host "Closing runner first-logon process: $($_.ProcessName) ($($_.Id))"
            Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
        }
}

# The Win32 shell cleanup lives in a test-only Go file, so it uses the native
# toolchain already built by CI and is never included in the shipped app.
$env:SIDELET_CI_PREPARE_DESKTOP = '1'
try {
    & go test -count=1 -timeout=30s -v ./cmd/sidelet -run '^TestPrepareWindowsCIDesktop$'
    if ($LASTEXITCODE -ne 0) { throw "Runner desktop preparation failed: $LASTEXITCODE" }
} finally {
    Remove-Item Env:SIDELET_CI_PREPARE_DESKTOP -ErrorAction SilentlyContinue
}
