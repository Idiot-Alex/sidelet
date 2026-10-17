param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
$ErrorActionPreference = 'Stop'
$SideletRoot = Split-Path $PSScriptRoot -Parent
Set-Location $SideletRoot
go test ./cmd/sidelet ./internal/...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go -C tools/windows-release run . -root $SideletRoot -arch $Arch -package
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
