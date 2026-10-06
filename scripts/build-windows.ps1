param([ValidateSet('amd64','arm64')][string]$Architecture = 'amd64')
$ErrorActionPreference = 'Stop'
$sideletRoot = Split-Path -Parent $PSScriptRoot
$sideletEnvironmentNames = @('GOMODCACHE','GOCACHE','GOOS','GOARCH','CGO_ENABLED')
$sideletPreviousEnvironment = @{}
foreach ($sideletName in $sideletEnvironmentNames) { $sideletPreviousEnvironment[$sideletName] = [Environment]::GetEnvironmentVariable($sideletName) }
Push-Location $sideletRoot
try {
    $env:GOMODCACHE = Join-Path $sideletRoot '.cache/go-mod'
    $env:GOCACHE = Join-Path $sideletRoot '.cache/go-build'
    Push-Location (Join-Path $sideletRoot 'frontend')
    try {
        & npm ci
        if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }
        & npm run build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
    } finally { Pop-Location }
    & go test ./internal/spike ./internal/storage ./internal/reminder ./internal/settings ./internal/exportdata ./internal/quickadd ./internal/integration
    if ($LASTEXITCODE -ne 0) { throw 'Go task tests failed' }
    $env:GOOS = 'windows'
    $env:GOARCH = $Architecture
    $env:CGO_ENABLED = '0'
    & go vet ./cmd/spike ./internal/platform
    if ($LASTEXITCODE -ne 0) { throw 'Windows vet failed' }
    New-Item -ItemType Directory -Force -Path 'build/bin' | Out-Null
    $sideletBinary = "build/bin/sidelet-spike-$Architecture.exe"
    & go build -trimpath '-ldflags=-H=windowsgui' -o $sideletBinary ./cmd/spike
    if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }
    & go version -m $sideletBinary
} finally {
    Pop-Location
    foreach ($sideletName in $sideletEnvironmentNames) { [Environment]::SetEnvironmentVariable($sideletName, $sideletPreviousEnvironment[$sideletName]) }
}
