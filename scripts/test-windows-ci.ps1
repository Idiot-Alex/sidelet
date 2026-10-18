param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
$SideletRoot = Split-Path $PSScriptRoot -Parent
Set-Location $SideletRoot
$Results = Join-Path $SideletRoot "build/results/windows-ci-$Arch"
New-Item -ItemType Directory -Force $Results | Out-Null

function Invoke-RecordedGo {
    param([string[]]$Arguments, [string]$Log)
    & go @Arguments 2>&1 | Tee-Object -FilePath (Join-Path $Results $Log)
    if ($LASTEXITCODE -ne 0) { throw "go $($Arguments -join ' ') failed: $LASTEXITCODE" }
}

$HostArch = (& go env GOHOSTARCH).Trim()
if ($LASTEXITCODE -ne 0 -or $HostArch -ne $Arch) {
    throw "Native Windows test requires a $Arch Go host; got $HostArch. Cross-compilation is not a runtime test."
}
if (-not $IsWindows) { throw 'Run this script with PowerShell 7 on Windows.' }

Invoke-RecordedGo -Arguments @('test','-json','./cmd/sidelet','./internal/...') -Log 'tests.jsonl'
Invoke-RecordedGo -Arguments @('vet','./cmd/sidelet','./internal/...') -Log 'vet.log'
Invoke-RecordedGo -Arguments @('-C','tools/windows-release','test','-json','./...') -Log 'release-tool-tests.jsonl'
Invoke-RecordedGo -Arguments @('-C','tools/windows-release','vet','./...') -Log 'release-tool-vet.log'

# Exercise the public PowerShell entry point, rather than duplicating its build.
& (Join-Path $PSScriptRoot 'package-windows.ps1') -Arch $Arch 2>&1 |
    Tee-Object -FilePath (Join-Path $Results 'package.log')
if ($LASTEXITCODE -ne 0) { throw "Windows package entry point failed: $LASTEXITCODE" }

$Info = Get-Content -Raw 'packaging/macos/app.json' | ConvertFrom-Json
$ZipName = "Sidelet-$($Info.version)-build$($Info.build)-windows-$Arch-local.zip"
$ZipPath = Join-Path $SideletRoot "build/releases/$ZipName"
$ExpectedHash = ((Get-Content -Raw "$ZipPath.sha256") -split '\s+')[0]
$ActualHash = (Get-FileHash -LiteralPath $ZipPath -Algorithm SHA256).Hash
if ($ActualHash -ne $ExpectedHash) { throw 'ZIP SHA-256 mismatch.' }
$Unpacked = Join-Path $Results 'unpacked'
Expand-Archive -LiteralPath $ZipPath -DestinationPath $Unpacked -Force
$PackageDir = Join-Path $Unpacked ([IO.Path]::GetFileNameWithoutExtension($ZipName))
$Exe = Join-Path $PackageDir 'Sidelet.exe'
$Metadata = Get-Content -Raw (Join-Path $PackageDir 'build-info.json') | ConvertFrom-Json
if ($Metadata.arch -ne $Arch -or $Metadata.version -ne $Info.version -or $Metadata.build -ne $Info.build) {
    throw 'Extracted archive metadata does not match this build.'
}
$Version = (Get-Item -LiteralPath $Exe).VersionInfo
if ($Version.ProductName -ne 'Sidelet' -or $Version.FileVersion -ne "$($Info.version).$($Info.build)") {
    throw 'Windows version APIs did not read the expected product/version resources.'
}
$Version | Format-List ProductName,FileVersion,ProductVersion,OriginalFilename |
    Out-File -FilePath (Join-Path $Results 'windows-version-info.txt') -Encoding utf8

# Only the verified executable extracted from the ZIP is launched. This test
# creates fresh synthetic profiles; it never reads the user's normal data.
$env:SIDELET_SMOKE_EXE = $Exe
$env:SIDELET_SMOKE_OUTPUT = Join-Path $Results 'smoke'
try {
    Invoke-RecordedGo -Arguments @('test','-count=1','-timeout=2m','-json','./cmd/sidelet','-run','^TestPackagedApplicationSmoke$') -Log 'packaged-smoke.jsonl'
} finally {
    Remove-Item Env:SIDELET_SMOKE_EXE -ErrorAction SilentlyContinue
    Remove-Item Env:SIDELET_SMOKE_OUTPUT -ErrorAction SilentlyContinue
}
$Summary = @"
## Windows $Arch
- Native tests, vet and PowerShell packaging passed.
- ZIP SHA-256 and Windows product/version API checks passed.
- The executable extracted from the ZIP started and exited twice.
- Synthetic SQLite tasks survived restart; startup cleanup persisted and the profile lock was released.
- Interactive mouse/IME/notification checks remain pending; the hosted runner runs with administrator privileges.
"@
$Summary | Out-File -FilePath (Join-Path $Results 'summary.md') -Encoding utf8
if ($env:GITHUB_STEP_SUMMARY) { $Summary | Out-File -FilePath $env:GITHUB_STEP_SUMMARY -Append -Encoding utf8 }
