param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
# This test uses the real per-user locations, only on a disposable CI runner.
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') { throw 'Installer lifecycle test requires a disposable hosted Windows GitHub Actions runner.' }
$SideletRoot = Split-Path $PSScriptRoot -Parent
. (Join-Path $PSScriptRoot 'windows-installer-common.ps1')
$Info = Get-Content -Raw (Join-Path $SideletRoot 'packaging/macos/app.json') | ConvertFrom-Json
$Name = "Sidelet-$($Info.version)-build$($Info.build)-windows-$Arch-local"
$Setup = Join-Path $SideletRoot "build/releases/$Name-setup.exe"
$Digest = ((Get-Content -Raw "$Setup.sha256") -split '\s+')[0]
if ((Get-FileHash $Setup -Algorithm SHA256).Hash -ne $Digest) { throw 'Installer digest mismatch.' }
$Results = Join-Path $SideletRoot "build/results/windows-ci-$Arch"
$Work = Join-Path $Results ('installer-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory $Work | Out-Null
$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs/Sidelet'
$Profile = Join-Path $env:APPDATA 'SideletMyGoLab'
$Shortcut = Join-Path ([Environment]::GetFolderPath('Programs')) 'Sidelet.lnk'
$RegPath = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\io.sidelet.mygo-lab'
$Registry = [Microsoft.Win32.RegistryKey]::OpenBaseKey('CurrentUser','Registry32')
$StartupRegistry = [Microsoft.Win32.RegistryKey]::OpenBaseKey('CurrentUser','Registry64')
$RunPath = 'Software\Microsoft\Windows\CurrentVersion\Run'
$ApprovalPath = 'Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run'
$StartupCommand = '"{0}" --mygo-opened-at-login' -f (Join-Path $InstallDir 'Sidelet.exe')
$DisabledApproval = [byte[]](3,0,0,0,11,12,13,14,15,16,17,18)
$UnrelatedStartupName = 'Sidelet-CI-unrelated-' + [guid]::NewGuid().ToString('N')
foreach ($Path in @($RunPath,$ApprovalPath)) {
    $Key = $StartupRegistry.OpenSubKey($Path)
    if ($null -ne $Key) {
        try {
            if ($Key.GetValueNames() -contains 'Sidelet') { throw 'Refusing to test over an existing Sidelet startup item.' }
        } finally { $Key.Dispose() }
    }
}
$ExistingKey = $Registry.OpenSubKey($RegPath)
if ($null -ne $ExistingKey -or (Test-Path $InstallDir) -or (Test-Path $Profile) -or (Test-Path $Shortcut)) {
    if ($null -ne $ExistingKey) { $ExistingKey.Dispose() }
    $Registry.Dispose()
    throw 'Refusing to test over an existing Sidelet installation, shortcut or profile.'
}

function Invoke-SetupProcess {
    param([string]$Exe, [string]$Arguments, [int]$Expected = 0)
    $Process = Start-Process -FilePath $Exe -ArgumentList $Arguments -PassThru
    try {
        if (-not $Process.WaitForExit(60000)) { $Process.Kill(); throw "Timed out: $Exe" }
        if ($Process.ExitCode -ne $Expected) { throw "$Exe returned $($Process.ExitCode), expected $Expected" }
    } finally { $Process.Dispose() }
}
function Get-InstalledValue {
    param([string]$Name)
    $Key = $Registry.OpenSubKey($RegPath)
    if ($null -eq $Key) { throw 'Uninstall registration missing.' }
    try { return $Key.GetValue($Name) } finally { $Key.Dispose() }
}
function Assert-ProfileHash {
    param([string]$Expected)
    if ((Get-FileHash (Join-Path $Profile 'sidelet-native.sqlite3')).Hash -ne $Expected) { throw 'Installer changed or removed SQLite user data.' }
}
function Get-StartupValue {
    param([string]$Path, [string]$Name = 'Sidelet')
    $Key = $StartupRegistry.OpenSubKey($Path)
    if ($null -eq $Key) { return $null }
    try { return ,$Key.GetValue($Name) } finally { $Key.Dispose() }
}
function Set-StartupFixture {
    param([string]$Command)
    $Key = $StartupRegistry.CreateSubKey($RunPath)
    try { $Key.SetValue('Sidelet',$Command,'String') } finally { $Key.Dispose() }
    $Key = $StartupRegistry.CreateSubKey($ApprovalPath)
    try { $Key.SetValue('Sidelet',$DisabledApproval,'Binary') } finally { $Key.Dispose() }
}
function Assert-StartupPreserved {
    param([string]$Command)
    if ((Get-StartupValue $RunPath) -cne $Command -or
        [Convert]::ToHexString([byte[]](Get-StartupValue $ApprovalPath)) -ne [Convert]::ToHexString($DisabledApproval)) {
        throw 'Installer changed the login command or system-disabled state.'
    }
}
function Copy-Uninstaller {
    $Copy = Join-Path $Work ('uninstall-' + [guid]::NewGuid().ToString('N') + '.exe')
    Copy-Item -LiteralPath (Join-Path $InstallDir 'Uninstall.exe') -Destination $Copy
    return $Copy
}
function Start-SyntheticApp {
    param([string]$Label, [string]$Duration = '3s')
    $Log = Join-Path $Work "$Label.log"
    $Output = Join-Path $Work $Label
    $Arguments = '-main -data-dir "{0}" -output "{1}" -log-file "{2}" -quit-after {3}' -f $Profile,$Output,$Log,$Duration
    return Start-Process -FilePath (Join-Path $InstallDir 'Sidelet.exe') -ArgumentList $Arguments -PassThru
}
function Assert-Startup {
    param([string]$Label)
    $Log = Join-Path $Work "$Label.log"
    $Entries = @(Get-Content -LiteralPath $Log | ForEach-Object {
        $Index = $_.IndexOf('{')
        if ($Index -ge 0) { $_.Substring($Index) | ConvertFrom-Json }
    })
    $Startup = @($Entries | Where-Object event -EQ 'startup-presentation')
    if ($Startup.Count -ne 1 -or $Startup[0].mainRenderer -ne 'native' -or -not $Startup[0].mainOpen) {
        throw "Installed application failed to reach native startup: $Label"
    }
    if ($Startup[0].taskSnapshot.tasks.Count -ne 2 -or ($Startup[0] | ConvertTo-Json -Depth 30) -notmatch 'CI 中文持久任务') {
        throw 'Installed application did not restore the synthetic tasks.'
    }
    foreach ($Entry in $Entries) {
        if ($Entry.PSObject.Properties.Name -contains 'storageError' -and $Entry.storageError) { throw $Entry.storageError }
    }
}

$OwnedProfile = $false
try {
    # Reuse the actual SQLite fixture previously validated by the packaged smoke
    # test. Copy all sidecars, and never seed/read a personal profile.
    $Seeds = @(Get-ChildItem -LiteralPath (Join-Path $Results 'smoke') -Filter sidelet-native.sqlite3 -Recurse |
        Where-Object { $_.Directory.Parent.Name -like 'packaged-smoke-*' })
    if ($Seeds.Count -ne 1) { throw 'Expected exactly one synthetic SQLite smoke fixture.' }
    Copy-Item -LiteralPath $Seeds[0].DirectoryName -Destination $Profile -Recurse
    $OwnedProfile = $true
    $Before = (Get-FileHash (Join-Path $Profile 'sidelet-native.sqlite3')).Hash
    $Payload = Join-Path $Results "unpacked/$Name"
    $OldPayload = Join-Path $Work 'previous-payload'
    Copy-Item -LiteralPath $Payload -Destination $OldPayload -Recurse
    # A synthetic prior package: same runnable app, distinguishable PE overlay,
    # and lower installer version. This tests replacement, not old-schema migration.
    $OldExe = Join-Path $OldPayload 'Sidelet.exe'
    $Stream = [IO.File]::Open($OldExe, [IO.FileMode]::Append)
    try { $Bytes = [Text.Encoding]::UTF8.GetBytes('Sidelet CI previous-package fixture'); $Stream.Write($Bytes,0,$Bytes.Length) } finally { $Stream.Dispose() }
    $Compiler = Get-SideletNSIS -Work $Work
    $OldSetup = Join-Path $Work 'previous-setup.exe'
    Invoke-SideletInstallerCompile -Compiler $Compiler -Root $SideletRoot -Payload $OldPayload `
        -Icon (Join-Path $SideletRoot "build/bin/Sidelet-$Arch.ico") -Output $OldSetup `
        -Arch $Arch -Version '0.1.0' -Build '1' -Script (Join-Path $Work 'previous.nsi')
    Invoke-SetupProcess -Exe $OldSetup -Arguments '/S'
    if ($null -ne (Get-StartupValue $RunPath)) { throw 'Fresh installation opted into login startup.' }
    Set-StartupFixture $StartupCommand
    $Key = $StartupRegistry.CreateSubKey($RunPath)
    try { $Key.SetValue($UnrelatedStartupName,'Sidelet CI sentinel','String') } finally { $Key.Dispose() }
    if ((Get-InstalledValue 'DisplayVersion') -ne '0.1.0.1') { throw 'Previous package registration incorrect.' }
    if ((Get-InstalledValue 'InstallLocation') -ne $InstallDir) { throw 'Incorrect per-user installation path.' }
    $Shell = New-Object -ComObject WScript.Shell
    if ($Shell.CreateShortcut($Shortcut).TargetPath -ne (Join-Path $InstallDir 'Sidelet.exe')) { throw 'Start Menu shortcut target incorrect.' }
    Assert-ProfileHash $Before
    $Sentinel = Join-Path $InstallDir 'user-created-file.txt'
    Set-Content -LiteralPath $Sentinel -Value 'must survive uninstall'

    # Real running EXE: installer AND uninstaller must fail safely, not kill it.
    $App = Start-SyntheticApp -Label 'running-upgrade' -Duration '15s'
    try {
        $Deadline = [DateTime]::UtcNow.AddSeconds(10)
        do {
            Start-Sleep -Milliseconds 200
            $LogPath = Join-Path $Work 'running-upgrade.log'
            $Ready = (Test-Path $LogPath) -and ((Get-Content -Raw $LogPath) -match 'startup-presentation')
        } until ($Ready -or $App.HasExited -or [DateTime]::UtcNow -gt $Deadline)
        if (-not $Ready -or $App.HasExited) { throw 'Synthetic app did not stay running for lock test.' }
        Invoke-SetupProcess -Exe $Setup -Arguments '/S' -Expected 10
        Invoke-SetupProcess -Exe (Copy-Uninstaller) -Arguments "/S _?=$InstallDir" -Expected 10
        if ($App.HasExited) { throw 'Setup terminated the running application.' }
        if ((Get-FileHash (Join-Path $InstallDir 'Sidelet.exe')).Hash -ne (Get-FileHash $OldExe).Hash) { throw 'Blocked upgrade replaced running EXE.' }
        if (-not $App.WaitForExit(30000) -or $App.ExitCode -ne 0) { throw 'Synthetic application did not exit normally.' }
    } finally {
        if (-not $App.HasExited) { $App.Kill() }
        $App.Dispose()
    }
    Assert-Startup 'running-upgrade'
    Assert-StartupPreserved $StartupCommand
    $Before = (Get-FileHash (Join-Path $Profile 'sidelet-native.sqlite3')).Hash
    Invoke-SetupProcess -Exe $Setup -Arguments '/S'
    Assert-StartupPreserved $StartupCommand
    Assert-ProfileHash $Before
    if ((Get-InstalledValue 'DisplayVersion') -ne "$($Info.version).$($Info.build)") { throw 'Upgrade version registration incorrect.' }
    if ((Get-FileHash (Join-Path $InstallDir 'Sidelet.exe')).Hash -ne (Get-FileHash (Join-Path $Payload 'Sidelet.exe')).Hash) { throw 'Upgrade did not install the verified payload.' }
    Invoke-SetupProcess -Exe $OldSetup -Arguments '/S' -Expected 14
    Assert-ProfileHash $Before
    Invoke-SetupProcess -Exe $Setup -Arguments '/S' # same-version repair
    Assert-StartupPreserved $StartupCommand
    Assert-ProfileHash $Before
    $App = Start-SyntheticApp -Label 'upgraded'
    try {
        if (-not $App.WaitForExit(45000) -or $App.ExitCode -ne 0) { throw 'Upgraded app failed.' }
    } finally { if (-not $App.HasExited) { $App.Kill() }; $App.Dispose() }
    Assert-Startup 'upgraded'
    $Before = (Get-FileHash (Join-Path $Profile 'sidelet-native.sqlite3')).Hash
    Invoke-SetupProcess -Exe (Copy-Uninstaller) -Arguments "/S _?=$InstallDir"
    if ($null -ne (Get-StartupValue $RunPath) -or $null -ne (Get-StartupValue $ApprovalPath)) { throw 'Uninstall left its startup registration.' }
    if ((Get-StartupValue $RunPath $UnrelatedStartupName) -ne 'Sidelet CI sentinel') { throw 'Uninstall removed another startup item.' }
    Assert-ProfileHash $Before
    foreach ($File in @('Sidelet.exe','Uninstall.exe','使用说明.txt','实机验收.md','build-info.json')) {
        if (Test-Path (Join-Path $InstallDir $File)) { throw "Uninstall left owned file: $File" }
    }
    if (-not (Test-Path $Sentinel)) { throw 'Uninstall removed an unrelated file.' }
    if (Test-Path $Shortcut) { throw 'Uninstall left Start Menu shortcut.' }
    $Key = $Registry.OpenSubKey($RegPath)
    if ($null -ne $Key) { $Key.Dispose(); throw 'Uninstall registration remains.' }
    Invoke-SetupProcess -Exe $Setup -Arguments '/S'
    if ($null -ne (Get-StartupValue $RunPath)) { throw 'Reinstall silently restored login startup from saved preferences.' }
    Assert-ProfileHash $Before
    $App = Start-SyntheticApp -Label 'reinstalled'
    try {
        if (-not $App.WaitForExit(45000) -or $App.ExitCode -ne 0) { throw 'Reinstalled app failed.' }
    } finally { if (-not $App.HasExited) { $App.Kill() }; $App.Dispose() }
    Assert-Startup 'reinstalled'
    # A portable copy may have claimed the shared name. Its state must survive.
    $OtherCommand = '"{0}" --mygo-opened-at-login' -f (Join-Path $Work 'portable copy/Sidelet.exe')
    Set-StartupFixture $OtherCommand
    Invoke-SetupProcess -Exe (Copy-Uninstaller) -Arguments "/S _?=$InstallDir"
    Assert-StartupPreserved $OtherCommand
    $Summary = @"
## Windows $Arch installer
- Per-user install, HKCU uninstall registration and Start Menu shortcut passed.
- Running-app upgrade/uninstall blocked without terminating the app.
- Synthetic previous-package upgrade replaced EXE; downgrade rejected; same-version repair passed.
- SQLite survived upgrade/uninstall/reinstall; installed app restored Chinese tasks.
- Uninstall removed only owned files; an unrelated file survived.
- Fresh install/reinstall did not enable login startup; upgrade/repair preserved the command and system-disabled state.
- Uninstall removed its own login registration and preserved another copy's registration and unrelated startup values.
- Installer is unsigned. Hosted runner is elevated; standard-user interactive acceptance remains pending.
"@
    $Summary | Out-File (Join-Path $Work 'summary.md') -Encoding utf8
    if ($env:GITHUB_STEP_SUMMARY) { $Summary | Out-File $env:GITHUB_STEP_SUMMARY -Append -Encoding utf8 }
} finally {
    foreach ($Path in @($RunPath,$ApprovalPath)) {
        $Key = $StartupRegistry.OpenSubKey($Path,$true)
        if ($null -ne $Key) {
            try { $Key.DeleteValue('Sidelet',$false); $Key.DeleteValue($UnrelatedStartupName,$false) } finally { $Key.Dispose() }
        }
    }
    $StartupRegistry.Dispose()
    $Registry.Dispose()
    if ($OwnedProfile -and (Test-Path $Profile)) {
        Copy-Item -LiteralPath $Profile -Destination (Join-Path $Work 'retained-profile') -Recurse
        Remove-Item -LiteralPath $Profile -Recurse -Force
    }
    # Installation files are left for diagnosis on failure; runner is disposable.
}
