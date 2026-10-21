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
!define MUI_WELCOMEPAGE_TEXT "$(Welcome)"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!define MUI_UNCONFIRMPAGE_TEXT_TOP "$(ConfirmRemove)"
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"
LangString Welcome ${LANG_ENGLISH} "Install Sidelet for your Windows account. No administrator permission is required.$\r$\n$\r$\nClose Sidelet before upgrading. Your tasks and settings are kept when upgrading or uninstalling."
LangString Welcome ${LANG_SIMPCHINESE} "将 Sidelet 安装到当前 Windows 账户，无需管理员权限。$\r$\n$\r$\n升级前请退出 Sidelet。升级和卸载都会保留任务与设置。"
LangString ConfirmRemove ${LANG_ENGLISH} "Remove Sidelet from this account? Tasks and settings in AppData will be kept. Close Sidelet before continuing."
LangString ConfirmRemove ${LANG_SIMPCHINESE} "要从当前账户卸载 Sidelet 吗？任务与设置将保留。请先退出 Sidelet。"
LangString Busy ${LANG_ENGLISH} "Another Sidelet installer is running."
LangString Busy ${LANG_SIMPCHINESE} "另一个 Sidelet 安装或卸载程序正在运行，请等待其完成。"
LangString WindowsVersion ${LANG_ENGLISH} "Sidelet requires Windows 10 or later."
LangString WindowsVersion ${LANG_SIMPCHINESE} "Sidelet 需要 Windows 10 或更新版本。"
LangString Windows64 ${LANG_ENGLISH} "Sidelet requires 64-bit Windows."
LangString Windows64 ${LANG_SIMPCHINESE} "Sidelet 需要 64 位 Windows。"
LangString UseX64 ${LANG_ENGLISH} "Use the x64 installer on this computer."
LangString UseX64 ${LANG_SIMPCHINESE} "请在此电脑上使用 x64 安装包。"
LangString CloseApp ${LANG_ENGLISH} "Please quit Sidelet before upgrading or uninstalling. Your data has not been changed."
LangString CloseApp ${LANG_SIMPCHINESE} "请先退出 Sidelet，再升级或卸载。安装程序尚未修改任务数据。"
LangString ExistingPath ${LANG_ENGLISH} "Sidelet is already installed in $0. Uninstall it before changing the installation folder."
LangString ExistingPath ${LANG_SIMPCHINESE} "Sidelet 已安装到 $0。更换安装目录前请先卸载，任务数据会保留。"
LangString OtherArch ${LANG_ENGLISH} "Uninstall the other architecture first. Your tasks will be kept."
LangString OtherArch ${LANG_SIMPCHINESE} "请先卸载另一种架构的 Sidelet，任务数据会保留。"
LangString Downgrade ${LANG_ENGLISH} "A newer Sidelet version is installed. Downgrades are not supported."
LangString Downgrade ${LANG_SIMPCHINESE} "已安装更新版本的 Sidelet，不支持覆盖降级。"
LangString InstallFailed ${LANG_ENGLISH} "Installation could not finish. Close Sidelet and run setup again."
LangString InstallFailed ${LANG_SIMPCHINESE} "安装未完成。请退出 Sidelet 后重新运行安装程序。"
LangString OldUninstaller ${LANG_ENGLISH} "This uninstaller no longer matches the registered installation."
LangString OldUninstaller ${LANG_SIMPCHINESE} "此卸载程序与当前安装记录不一致，请从 Windows 应用列表卸载 Sidelet。"
LangString RemoveFailed ${LANG_ENGLISH} "Uninstall could not finish. Close Sidelet and try again. Your tasks are kept."
LangString RemoveFailed ${LANG_SIMPCHINESE} "卸载未完成。请退出 Sidelet 后重试，任务数据会保留。"
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
    MessageBox MB_OK|MB_ICONEXCLAMATION "$(Busy)" /SD IDOK
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
    MessageBox MB_OK|MB_ICONSTOP "$(WindowsVersion)" /SD IDOK
    SetErrorLevel 12
    Quit
  ${EndIf}
  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "$(Windows64)" /SD IDOK
    SetErrorLevel 12
    Quit
  ${EndIf}
!if "${ARCH}" == "arm64"
  ${IfNot} ${IsNativeARM64}
    MessageBox MB_OK|MB_ICONSTOP "$(UseX64)" /SD IDOK
    SetErrorLevel 12
    Quit
  ${EndIf}
!endif
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Local\Sidelet.Setup") p .r0 ?e'
  Pop $1
  StrCpy $SetupMutex $0
  ${If} $0 == 0
  ${OrIf} $1 == 183
    MessageBox MB_OK|MB_ICONEXCLAMATION "$(Busy)" /SD IDOK
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
    MessageBox MB_OK|MB_ICONEXCLAMATION "$(CloseApp)" /SD IDOK
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
    MessageBox MB_OK|MB_ICONSTOP "$(ExistingPath)" /SD IDOK
    SetErrorLevel 13
    Quit
  ${EndIf}
  ReadRegStr $0 HKCU "${APPKEY}" "Architecture"
  ${If} $0 != ""
  ${AndIf} $0 != "${ARCH}"
    MessageBox MB_OK|MB_ICONSTOP "$(OtherArch)" /SD IDOK
    SetErrorLevel 13
    Quit
  ${EndIf}
  ReadRegStr $0 HKCU "${APPKEY}" "DisplayVersion"
  ${If} $0 != ""
    ${VersionCompare} "$0" "${VERSION}.${BUILD}" $1
    ${If} $1 == 1
      MessageBox MB_OK|MB_ICONSTOP "$(Downgrade)" /SD IDOK
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
  WriteRegStr HKCU "${APPKEY}" "DisplayIcon" '"$INSTDIR\Sidelet.exe",0'
  WriteRegStr HKCU "${APPKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${APPKEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${APPKEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "${APPKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${APPKEY}" "NoRepair" 1
  IfErrors failed
  SetErrorLevel 0
  Goto finished
  failed:
    MessageBox MB_OK|MB_ICONSTOP "$(InstallFailed)" /SD IDOK
    SetErrorLevel 2
    Abort
  finished:
SectionEnd

Section "Uninstall"
  ReadRegStr $0 HKCU "${APPKEY}" "InstallLocation"
  ${If} $0 != $INSTDIR
    MessageBox MB_OK|MB_ICONSTOP "$(OldUninstaller)" /SD IDOK
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
  IfErrors failed
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
    MessageBox MB_OK|MB_ICONSTOP "$(RemoveFailed)" /SD IDOK
    SetErrorLevel 2
    Abort
  finished:
SectionEnd
