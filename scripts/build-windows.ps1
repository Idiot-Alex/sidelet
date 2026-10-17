param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
$ErrorActionPreference = 'Stop'
$SideletRoot = Split-Path $PSScriptRoot -Parent
Set-Location $SideletRoot
go -C tools/windows-release run . -root $SideletRoot -arch $Arch
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
