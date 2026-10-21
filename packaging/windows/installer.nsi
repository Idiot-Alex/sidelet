; -*- coding: utf-8 -*-
; Compile through scripts/build-windows-installer.ps1 with verified ZIP inputs.
Unicode true
RequestExecutionLevel user
ManifestDPIAware true
CRCCheck force
SetCompressor /SOLID lzma
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "WordFunc.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!define APPKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\io.sidelet.mygo-lab"
Name "Sidelet"
OutFile "${OUTPUT}"
InstallDir "$LOCALAPPDATA\Programs\Sidelet"
InstallDirRegKey HKCU "${APPKEY}" "InstallLocation"
VIProductVersion "${VERSION}.${BUILD}"
VIAddVersionKey /LANG=1033 "ProductName" "Sidelet"
VIAddVersionKey /LANG=1033 "FileDescription" "Sidelet Setup"
VIAddVersionKey /LANG=1033 "FileVersion" "${VERSION}.${BUILD}"
VIAddVersionKey /LANG=1033 "LegalCopyright" "Sidelet contributors"
!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TEXT "Install Sidelet for your Windows account. No administrator permission is required.$\r$\n$\r$\nClose Sidelet before upgrading. Your tasks and settings are kept when upgrading or uninstalling."
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!define MUI_UNCONFIRMPAGE_TEXT_TOP "Remove Sidelet from this account? Tasks and settings in AppData will be kept. Close Sidelet before continuing."
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"
Var SetupMutex

; Shared by setup and uninstall. Keep the mutex handle for the process lifetime.
!macro Init PREFIX
Function ${PREFIX}.onInit
  SetShellVarContext current
  SetRegView 32
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Local\Sidelet.Setup") p .r0 ?e'
  Pop $1
  StrCpy $SetupMutex $0
  ${If} $0 == 0
  ${OrIf} $1 == 183
    MessageBox MB_OK|MB_ICONEXCLAMATION "Another Sidelet installer is running." /SD IDOK
    SetErrorLevel 11
    Quit
  ${EndIf}
FunctionEnd
!macroend
!insertmacro Init "un"

Function .onInit
  SetShellVarContext current
  SetRegView 32
  ${IfNot} ${AtLeastWin10}
    MessageBox MB_OK|MB_ICONSTOP "Sidelet requires Windows 10 or later." /SD IDOK
    SetErrorLevel 12
    Quit
  ${EndIf}
  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "Sidelet requires 64-bit Windows." /SD IDOK
    SetErrorLevel 12
    Quit
  ${EndIf}
!if "${ARCH}" == "arm64"
  ${IfNot} ${IsNativeARM64}
    MessageBox MB_OK|MB_ICONSTOP "Use the x64 installer on this computer." /SD IDOK
    SetErrorLevel 12
    Quit
  ${EndIf}
!endif
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Local\Sidelet.Setup") p .r0 ?e'
  Pop $1
  StrCpy $SetupMutex $0
  ${If} $0 == 0
  ${OrIf} $1 == 183
    MessageBox MB_OK|MB_ICONEXCLAMATION "Another Sidelet installer is running." /SD IDOK
    SetErrorLevel 11
    Quit
  ${EndIf}
FunctionEnd

; OPEN_EXISTING + GENERIC_WRITE + no sharing: reject a running/mapped or locked
; executable without truncating it. Never terminate a user's process.
!macro CheckClosed
  IfFileExists "$INSTDIR\Sidelet.exe" 0 closed
  System::Call 'kernel32::CreateFileW(w "$INSTDIR\Sidelet.exe", i 0x40000000, i 0, p 0, i 3, i 0, p 0) p .r0'
  ${If} $0 == -1
    MessageBox MB_OK|MB_ICONEXCLAMATION "Please quit Sidelet before upgrading or uninstalling. Your data has not been changed." /SD IDOK
    SetErrorLevel 10
    Quit
  ${EndIf}
  System::Call 'kernel32::CloseHandle(p r0)'
  closed:
!macroend

Section "Install"
  ; A second installation path would leave an orphaned uninstaller/shortcut.
  ReadRegStr $0 HKCU "${APPKEY}" "InstallLocation"
  ${If} $0 != ""
  ${AndIf} $0 != $INSTDIR
    MessageBox MB_OK|MB_ICONSTOP "Sidelet is already installed in $0. Uninstall it before changing the installation folder." /SD IDOK
    SetErrorLevel 13
    Quit
  ${EndIf}
  ReadRegStr $0 HKCU "${APPKEY}" "Architecture"
  ${If} $0 != ""
  ${AndIf} $0 != "${ARCH}"
    MessageBox MB_OK|MB_ICONSTOP "Uninstall the other architecture first. Your tasks will be kept." /SD IDOK
    SetErrorLevel 13
    Quit
  ${EndIf}
  ReadRegStr $0 HKCU "${APPKEY}" "DisplayVersion"
  ${If} $0 != ""
    ${VersionCompare} "$0" "${VERSION}.${BUILD}" $1
    ${If} $1 == 1
      MessageBox MB_OK|MB_ICONSTOP "A newer Sidelet version is installed. Downgrades are not supported." /SD IDOK
      SetErrorLevel 14
      Quit
    ${EndIf}
  ${EndIf}
  !insertmacro CheckClosed
  SetOutPath "$INSTDIR"
  SetOverwrite on
  ClearErrors
  File "${PAYLOAD}\Sidelet.exe"
  File "${PAYLOAD}\使用说明.txt"
  File "${PAYLOAD}\实机验收.md"
  File "${PAYLOAD}\build-info.json"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  IfErrors failed
  CreateShortCut "$SMPROGRAMS\Sidelet.lnk" "$INSTDIR\Sidelet.exe"
  WriteRegStr HKCU "${APPKEY}" "DisplayName" "Sidelet"
  WriteRegStr HKCU "${APPKEY}" "Publisher" "Sidelet"
  WriteRegStr HKCU "${APPKEY}" "DisplayVersion" "${VERSION}.${BUILD}"
  WriteRegStr HKCU "${APPKEY}" "Architecture" "${ARCH}"
  WriteRegStr HKCU "${APPKEY}" "DisplayIcon" '$"$INSTDIR\Sidelet.exe$",0'
  WriteRegStr HKCU "${APPKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${APPKEY}" "UninstallString" '$"$INSTDIR\Uninstall.exe$"'
  WriteRegStr HKCU "${APPKEY}" "QuietUninstallString" '$"$INSTDIR\Uninstall.exe$" /S'
  WriteRegDWORD HKCU "${APPKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${APPKEY}" "NoRepair" 1
  IfErrors failed
  SetErrorLevel 0
  Goto finished
  failed:
    MessageBox MB_OK|MB_ICONSTOP "Installation could not finish. Close Sidelet and run setup again." /SD IDOK
    SetErrorLevel 2
    Abort
  finished:
SectionEnd

Section "Uninstall"
  ReadRegStr $0 HKCU "${APPKEY}" "InstallLocation"
  ${If} $0 != $INSTDIR
    MessageBox MB_OK|MB_ICONSTOP "This uninstaller no longer matches the registered installation." /SD IDOK
    SetErrorLevel 13
    Quit
  ${EndIf}
  !insertmacro CheckClosed
  ClearErrors
  Delete "$INSTDIR\Sidelet.exe"
  IfErrors failed
  Delete "$INSTDIR\使用说明.txt"
  Delete "$INSTDIR\实机验收.md"
  Delete "$INSTDIR\build-info.json"
  ; Explorer can briefly hold a new shortcut open.
  StrCpy $1 50
  retryShortcut:
    ClearErrors
    Delete "$SMPROGRAMS\Sidelet.lnk"
    IfErrors 0 shortcutRemoved
    IntOp $1 $1 - 1
    IntCmp $1 0 failed
    Sleep 100
    Goto retryShortcut
  shortcutRemoved:
  DeleteRegKey HKCU "${APPKEY}"
  IfErrors failed
  Delete "$INSTDIR\Uninstall.exe"
  ; Never recursively remove this directory or touch AppData/user files.
  RMDir "$INSTDIR"
  SetErrorLevel 0
  Goto finished
  failed:
    MessageBox MB_OK|MB_ICONSTOP "Uninstall could not finish. Close Sidelet and try again. Your tasks are kept." /SD IDOK
    SetErrorLevel 2
    Abort
  finished:
SectionEnd
