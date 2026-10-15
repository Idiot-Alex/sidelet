param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$env:GOOS = 'windows'
$env:GOARCH = $Arch
$env:CGO_ENABLED = '0'
New-Item -ItemType Directory -Force build/bin | Out-Null
go build -trimpath -ldflags='-s -w -H=windowsgui' -o "build/bin/Sidelet-$Arch.exe" ./cmd/sidelet
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
