param(
    [Parameter(Mandatory)][ValidatePattern('^Sidelet CI reminder [0-9a-f]{16}$')][string]$Title,
    [Parameter(Mandatory)][string]$OutputPath
)
$ErrorActionPreference = 'Stop'
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') {
    throw 'Notification probing is restricted to disposable GitHub-hosted desktops.'
}
# Inspect only Shell windows in this session, match the random synthetic title,
# and invoke only its nearest button ancestor. Never open Notification Center,
# change settings, enumerate notification text, or synthesize an app callback.
$Result = @{ found = $false; invoked = $false; reason = 'No matching visible banner in this runner session.' }
try {
    try {
        Add-Type -AssemblyName UIAutomationClient
        Add-Type -AssemblyName UIAutomationTypes
    } catch {
        # PowerShell Core may not search the installed Windows Desktop runtime.
        # Use its matching major version; do not install or change any runtime.
        $Desktop = Join-Path $env:ProgramFiles 'dotnet\shared\Microsoft.WindowsDesktop.App'
        $Runtime = Get-ChildItem -LiteralPath $Desktop -Directory -ErrorAction Stop |
            Where-Object { $_.Name -match ('^' + [Environment]::Version.Major + '\.') } |
            Sort-Object { [version]$_.Name } -Descending | Select-Object -First 1
        if ($null -eq $Runtime) { throw 'No matching Windows Desktop runtime for Shell UI Automation.' }
        foreach ($Assembly in 'WindowsBase.dll','UIAutomationTypes.dll','UIAutomationClient.dll') {
            [void][Reflection.Assembly]::LoadFrom((Join-Path $Runtime.FullName $Assembly))
        }
    }
    $Automation = [System.Windows.Automation.AutomationElement]
    $Scope = [System.Windows.Automation.TreeScope]
    $Condition = [System.Windows.Automation.PropertyCondition]::new($Automation::NameProperty, $Title)
    $Walker = [System.Windows.Automation.TreeWalker]::RawViewWalker
    $End = [DateTime]::UtcNow.AddSeconds(5)
    $Session = (Get-Process -Id $PID).SessionId
    do {
        $Roots = $Automation::RootElement.FindAll($Scope::Children, [System.Windows.Automation.Condition]::TrueCondition)
        foreach ($Window in $Roots) {
            $Owner = Get-Process -Id $Window.Current.ProcessId -ErrorAction SilentlyContinue
            if ($null -eq $Owner -or $Owner.SessionId -ne $Session -or
                $Owner.ProcessName -notin @('explorer', 'ShellExperienceHost', 'ShellHost')) { continue }
            $Match = $Window.FindFirst($Scope::Descendants, $Condition)
            if ($null -eq $Match -or $Match.Current.IsOffscreen) { continue }
            $Result.found = $true
            $Result.reason = 'Matching banner visible; no supported activation pattern.'
            $Element = $Match
            for ($Depth = 0; $Depth -lt 6 -and $null -ne $Element; $Depth++) {
                if ($Element.Current.ProcessId -ne $Owner.Id) { break }
                $Pattern = $null
                if ($Element.Current.ControlType -eq [System.Windows.Automation.ControlType]::Button -and
                    $Element.TryGetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern, [ref]$Pattern)) {
                    $Pattern.Invoke()
                    $Result.invoked = $true
                    $Result.reason = 'Invoked the visible synthetic notification through UI Automation.'
                    break
                }
                $Element = $Walker.GetParent($Element)
            }
            break
        }
        if ($Result.found) { break }
        Start-Sleep -Milliseconds 200
    } while ([DateTime]::UtcNow -lt $End)
} catch {
    $Result.reason = 'Shell UI Automation unavailable: ' + $_.Exception.Message
} finally {
    [IO.File]::WriteAllText($OutputPath, ($Result | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
}
