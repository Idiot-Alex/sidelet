# Only the disposable GitHub-hosted ARM64 image. Never use on a personal or
# self-hosted machine. The image can start with OOBE above even topmost windows:
# https://github.com/actions/runner-images/issues/14069
$ErrorActionPreference = 'Stop'
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or
    $env:RUNNER_ENVIRONMENT -ne 'github-hosted' -or $env:RUNNER_ARCH -ne 'ARM64') {
    throw 'Desktop preparation is restricted to disposable GitHub-hosted Windows ARM64 runners.'
}

$OobePolicy = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\OOBE'
New-Item -Path $OobePolicy -Force | Out-Null
Set-ItemProperty -Path $OobePolicy -Name DisablePrivacyExperience -Type DWord -Value 1
$Engagement = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\UserProfileEngagement'
New-Item -Path $Engagement -Force | Out-Null
Set-ItemProperty -Path $Engagement -Name ScoobeSystemSettingEnabled -Type DWord -Value 0
$TestSession = (Get-Process -Id $PID).SessionId
foreach ($Name in 'UserOOBEBroker','WWAHost','SystemPropertiesPerformance') {
    Get-Process -Name $Name -ErrorAction SilentlyContinue |
        Where-Object { $_.SessionId -eq $TestSession } | ForEach-Object {
            Write-Host "Closing runner first-logon process: $($_.ProcessName) ($($_.Id))"
            Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
        }
}

Add-Type -TypeDefinition @'
using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
public static class SideletCIDesktop {
    [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
    [DllImport("user32.dll")] static extern void keybd_event(byte key, byte scan, uint flags, UIntPtr extra);
    [DllImport("user32.dll")] static extern bool ShowWindow(IntPtr window, int command);
    public static void DismissStart() {
        IntPtr window = GetForegroundWindow();
        uint id;
        GetWindowThreadProcessId(window, out id);
        if (id == 0) return;
        try {
            string name = Process.GetProcessById((int)id).ProcessName;
            // Escape is sent only while the runner's Start menu has focus.
            if (name == "StartMenuExperienceHost") {
                keybd_event(0x1b, 0, 0, UIntPtr.Zero);
                keybd_event(0x1b, 0, 2, UIntPtr.Zero);
                Console.WriteLine("Dismissed runner Start menu");
            }
            // A first-logon WSL update terminal can also cover the fixtures.
            // Minimize it; do not stop the runner or its shell processes.
            if (name == "WindowsTerminal") {
                ShowWindow(window, 6);
                Console.WriteLine("Minimized runner terminal");
            }
        } catch (ArgumentException) { /* The transient window already closed. */ }
    }
}
'@
for ($Pass = 0; $Pass -lt 3; $Pass++) {
    Start-Sleep -Milliseconds 400
    [SideletCIDesktop]::DismissStart()
}
