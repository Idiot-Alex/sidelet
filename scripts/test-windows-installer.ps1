param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') {
    throw 'Installer acceptance requires a disposable hosted Windows runner.'
}
$SideletRoot = Split-Path $PSScriptRoot -Parent
Set-Location $SideletRoot
$HostArch = (& go env GOHOSTARCH).Trim()
if ($LASTEXITCODE -ne 0 -or $HostArch -ne $Arch) { throw 'Permission checks require the native Go architecture.' }
& go test ./tools/windows-ci
if ($LASTEXITCODE -ne 0) { throw 'Permission helper tests failed.' }
& go vet ./tools/windows-ci
if ($LASTEXITCODE -ne 0) { throw 'Permission helper vet failed.' }
& go build -o build/bin/windows-ci.exe ./tools/windows-ci
if ($LASTEXITCODE -ne 0) { throw 'Permission helper build failed.' }
& go test -c -o "build/bin/sidelet-smoke-$Arch.test.exe" ./cmd/sidelet
if ($LASTEXITCODE -ne 0) { throw 'Smoke harness build failed.' }
& (Join-Path $SideletRoot 'build/bin/windows-ci.exe') suite $Arch $SideletRoot
if ($LASTEXITCODE -ne 0) { throw 'Non-admin acceptance failed.' }
