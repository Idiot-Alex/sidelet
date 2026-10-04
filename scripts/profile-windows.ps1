param(
    [Parameter(Mandatory = $true)][int]$ProcessId,
    [ValidateRange(10,86400)][int]$DurationSeconds = 600,
    [ValidateRange(1,60)][int]$IntervalSeconds = 5,
    [string]$OutputDirectory = 'build/results'
)
$ErrorActionPreference = 'Stop'
$sideletTarget = Get-Process -Id $ProcessId -ErrorAction Stop
$sideletStamp = Get-Date -Format 'yyyyMMdd-HHmmss'
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$sideletCsv = Join-Path $OutputDirectory "samples-$sideletStamp.csv"
$sideletEnvironment = Join-Path $OutputDirectory "environment-$sideletStamp.json"
$sideletOs = Get-CimInstance Win32_OperatingSystem
$sideletCpu = Get-CimInstance Win32_Processor
$sideletLogicalCores = (Get-CimInstance Win32_ComputerSystem).NumberOfLogicalProcessors
Add-Type -AssemblyName System.Windows.Forms
$sideletScreens = @([System.Windows.Forms.Screen]::AllScreens | ForEach-Object {
    @{ DeviceName = $_.DeviceName; Primary = $_.Primary; BoundsReportedByWinForms = $_.Bounds.ToString(); WorkAreaReportedByWinForms = $_.WorkingArea.ToString() }
})
@{
    CollectedAt = (Get-Date).ToString('o')
    Windows = $sideletOs.Caption
    WindowsVersion = $sideletOs.Version
    OSArchitecture = $sideletOs.OSArchitecture
    Cpu = @($sideletCpu.Name)
    LogicalCores = $sideletLogicalCores
    RootProcessId = $ProcessId
    Binary = $sideletTarget.Path
    BinarySHA256 = (Get-FileHash $sideletTarget.Path -Algorithm SHA256).Hash
    Screens = $sideletScreens
    DPI = 'Use dpi= and physical WorkArea in the Spike log; WinForms coordinate units depend on the collector process.'
    CPUConvention = 'Process tree CPU delta / elapsed wall time / logical cores * 100'
} | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 $sideletEnvironment
$sideletPreviousCpu = @{}
$sideletPreviousAt = Get-Date
$sideletEnd = (Get-Date).AddSeconds($DurationSeconds)
while ((Get-Date) -lt $sideletEnd) {
    if (-not (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)) { break }
    $sideletProcesses = @(Get-CimInstance Win32_Process)
    $sideletIds = [System.Collections.Generic.HashSet[int]]::new()
    [void]$sideletIds.Add($ProcessId)
    do {
        $sideletAdded = $false
        foreach ($sideletProcess in $sideletProcesses) {
            if ($sideletIds.Contains([int]$sideletProcess.ParentProcessId)) {
                if ($sideletIds.Add([int]$sideletProcess.ProcessId)) { $sideletAdded = $true }
            }
        }
    } while ($sideletAdded)
    $sideletMembers = @(Get-Process -Id @($sideletIds) -ErrorAction SilentlyContinue)
    $sideletAt = Get-Date
    $sideletElapsed = ($sideletAt - $sideletPreviousAt).TotalSeconds
    $sideletCpuDelta = 0.0
    $sideletCurrentCpu = @{}
    foreach ($sideletMember in $sideletMembers) {
        $sideletCurrentCpu[$sideletMember.Id] = $sideletMember.CPU
        if ($sideletPreviousCpu.ContainsKey($sideletMember.Id)) {
            $sideletCpuDelta += [Math]::Max(0, $sideletMember.CPU - $sideletPreviousCpu[$sideletMember.Id])
        }
    }
    $sideletRootWorkingSet = ($sideletMembers | Where-Object Id -eq $ProcessId | Measure-Object WorkingSet64 -Sum).Sum
    $sideletWebViewWorkingSet = ($sideletMembers | Where-Object ProcessName -eq 'msedgewebview2' | Measure-Object WorkingSet64 -Sum).Sum
    $sideletPrivate = ($sideletMembers | Measure-Object PrivateMemorySize64 -Sum).Sum
    [pscustomobject]@{
        Timestamp = $sideletAt.ToString('o')
        ProcessCount = $sideletMembers.Count
        RootWorkingSetBytes = $sideletRootWorkingSet
        WebView2WorkingSetBytes = $sideletWebViewWorkingSet
        TotalPrivateBytes = $sideletPrivate
        CPUPercentNormalized = if ($sideletPreviousCpu.Count -gt 0) { [Math]::Round(100 * $sideletCpuDelta / $sideletElapsed / $sideletLogicalCores, 4) } else { $null }
        WebView2Versions = (@($sideletMembers | Where-Object ProcessName -eq 'msedgewebview2' | ForEach-Object { $_.FileVersion } | Sort-Object -Unique) -join ';')
    } | Export-Csv -NoTypeInformation -Append -Encoding UTF8 $sideletCsv
    $sideletPreviousCpu = $sideletCurrentCpu
    $sideletPreviousAt = $sideletAt
    Write-Host "$($sideletAt.ToString('HH:mm:ss')) processes=$($sideletMembers.Count) privateMB=$([Math]::Round($sideletPrivate/1MB,1))"
    Start-Sleep -Seconds $IntervalSeconds
}
Write-Host "Saved $sideletCsv and $sideletEnvironment"
