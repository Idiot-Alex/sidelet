param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64', [switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
if (-not $IsWindows) { throw 'Run with PowerShell 7 on Windows; use Windows CI from a Mac.' }
$SideletRoot = Split-Path $PSScriptRoot -Parent
. (Join-Path $PSScriptRoot 'windows-installer-common.ps1')
if (-not $SkipBuild) {
    & (Join-Path $PSScriptRoot 'package-windows.ps1') -Arch $Arch
    if ($LASTEXITCODE -ne 0) { throw "Packaging failed: $LASTEXITCODE" }
}
$Info = Get-Content -Raw (Join-Path $SideletRoot 'packaging/macos/app.json') | ConvertFrom-Json
$Name = "Sidelet-$($Info.version)-build$($Info.build)-windows-$Arch-local"
$ReleaseDir = Join-Path $SideletRoot 'build/releases'
$ZipPath = Join-Path $ReleaseDir "$Name.zip"
$Checksum = ((Get-Content -Raw "$ZipPath.sha256") -split '\s+')[0]
if ($Checksum -notmatch '^[a-fA-F0-9]{64}$' -or (Get-FileHash $ZipPath -Algorithm SHA256).Hash -ne $Checksum) {
    throw 'Input ZIP failed SHA-256 verification.'
}
$Work = Join-Path $ReleaseDir ('.installer-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory $Work | Out-Null
try {
    $Payload = Join-Path $Work 'payload'
    New-Item -ItemType Directory $Payload | Out-Null
    $Files = @('Sidelet.exe','使用说明.txt','实机验收.md','build-info.json')
    $Zip = [IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        if ($Zip.Entries.Count -ne $Files.Count) { throw 'Unexpected installer payload entries.' }
        foreach ($File in $Files) {
            $Entries = @($Zip.Entries | Where-Object FullName -CEQ "$Name/$File")
            if ($Entries.Count -ne 1) { throw "Missing/duplicate payload: $File" }
            [IO.Compression.ZipFileExtensions]::ExtractToFile($Entries[0], (Join-Path $Payload $File))
        }
    } finally { $Zip.Dispose() }
    $Metadata = Get-Content -Raw (Join-Path $Payload 'build-info.json') | ConvertFrom-Json
    if ($Metadata.arch -ne $Arch -or $Metadata.version -ne $Info.version -or $Metadata.build -ne $Info.build) {
        throw 'ZIP metadata does not match the requested installer.'
    }
    $ExeVersion = (Get-Item (Join-Path $Payload 'Sidelet.exe')).VersionInfo
    if ($ExeVersion.ProductName -ne 'Sidelet' -or $ExeVersion.FileVersion -ne "$($Info.version).$($Info.build)") {
        throw 'Input executable version mismatch.'
    }
    $Compiler = Get-SideletNSIS -Work $Work
    $SetupName = "$Name-setup.exe"
    $Staged = Join-Path $Work $SetupName
    Invoke-SideletInstallerCompile -Compiler $Compiler -Root $SideletRoot -Payload $Payload `
        -Icon (Join-Path $SideletRoot "build/bin/Sidelet-$Arch.ico") -Output $Staged `
        -Arch $Arch -Version $Info.version -Build $Info.build -Script (Join-Path $Work 'build.nsi')
    $Hash = (Get-FileHash $Staged -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText("$Staged.sha256", "$Hash  $SetupName`n", [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $Staged -Destination (Join-Path $ReleaseDir $SetupName) -Force
    Move-Item -LiteralPath "$Staged.sha256" -Destination (Join-Path $ReleaseDir "$SetupName.sha256") -Force
    Write-Host "Created and verified $SetupName ($Hash)"
} finally {
    Remove-Item -LiteralPath $Work -Recurse -Force
}
