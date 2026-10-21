# Shared build helpers. No installer is executed by this file.
Set-StrictMode -Version Latest

function Get-SideletNSIS {
    param([Parameter(Mandatory)][string]$Work)
    # Same pinned official archive as MyGo 0.5.1; no machine-wide installation.
    $Digest = 'ba63dffc4410ee89193e1cb5a41989991bd77c61068da17e3156d136b7b0b3d8'
    $Archive = Join-Path $Work 'nsis.zip'
    $Urls = @(
        'https://github.com/egoist/mygo/releases/download/nsis-3.13/nsis-3.13.zip',
        'https://downloads.sourceforge.net/project/nsis/NSIS%203/3.13/nsis-3.13.zip'
    )
    $Downloaded = $false
    foreach ($Url in $Urls) {
        try {
            Invoke-WebRequest -Uri $Url -OutFile $Archive -TimeoutSec 120
            if ((Get-FileHash $Archive -Algorithm SHA256).Hash -ne $Digest) { throw 'NSIS archive SHA-256 mismatch.' }
            $Downloaded = $true
            break
        } catch { Write-Warning "NSIS download failed: $($_.Exception.Message)" }
    }
    if (-not $Downloaded) { throw 'Unable to obtain the pinned NSIS compiler.' }
    # The digest authenticates this fixed archive before extracting/executing it.
    Expand-Archive -LiteralPath $Archive -DestinationPath (Join-Path $Work 'nsis')
    $Compiler = Join-Path $Work 'nsis/nsis-3.13/makensis.exe'
    if (-not (Test-Path -LiteralPath $Compiler)) { throw 'Pinned archive has no expected NSIS compiler.' }
    return $Compiler
}

function ConvertTo-NSISString {
    param([string]$Value)
    return '"' + $Value.Replace('$', '$$').Replace('"', '$\"').Replace("`r", '$\r').Replace("`n", '$\n') + '"'
}

function Invoke-SideletInstallerCompile {
    param([string]$Compiler, [string]$Root, [string]$Payload, [string]$Icon,
        [string]$Output, [string]$Arch, [string]$Version, [string]$Build, [string]$Script)
    $Defines = [ordered]@{ PAYLOAD=$Payload; ICON=$Icon; OUTPUT=$Output; ARCH=$Arch; VERSION=$Version; BUILD=$Build }
    $Lines = @('; -*- coding: utf-8 -*-')
    foreach ($Key in $Defines.Keys) { $Lines += '!define ' + $Key + ' ' + (ConvertTo-NSISString $Defines[$Key]) }
    $Lines += '!include ' + (ConvertTo-NSISString (Join-Path $Root 'packaging/windows/installer.nsi'))
    [IO.File]::WriteAllLines($Script, $Lines, [Text.UTF8Encoding]::new($false))
    & $Compiler /NOCONFIG /INPUTCHARSET UTF8 /V3 /WX $Script | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "NSIS compiler failed: $LASTEXITCODE" }
    if (-not (Test-Path -LiteralPath $Output)) { throw 'NSIS did not produce an installer.' }
    $Actual = (Get-Item -LiteralPath $Output).VersionInfo
    if ($Actual.ProductName -ne 'Sidelet' -or $Actual.FileVersion -ne "$Version.$Build") {
        throw 'Installer version resource mismatch.'
    }
}
