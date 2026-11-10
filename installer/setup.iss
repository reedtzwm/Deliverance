; Inno Setup — один Setup.exe: ставит программу, ярлыки, запись об удалении.
; Файлы zapret приложение скачивает само в %ProgramData%\DZapret.
#define AppName "DZapret"
#ifndef AppVer
  #define AppVer "1.8.0"
#endif
#define AppExe  "dzapret.exe"

[Setup]
; AppId прежний — новая версия встаёт поверх установленной (программа раньше называлась Zapret GUI, затем Deliverance)
AppId={{B7E2C4A1-5D3F-4E8A-9C61-0A1B2C3D4E5F}
AppName={#AppName}
AppVersion={#AppVer}
AppPublisher=DZapret
DefaultDirName={autopf}\{#AppName}
UsePreviousAppDir=no
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\build\installer
OutputBaseFilename=DZapret-Setup-{#AppVer}
SetupIconFile=..\build\windows\icon.ico
UninstallDisplayIcon={app}\{#AppExe}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
; checkedonce: при первой установке галочки стоят, а при обновлении (в том числе тихом, из программы)
; сохраняется прежний выбор пользователя — отключённый автозапуск не включится сам
Name: "desktopicon"; Description: "Создать ярлык на рабочем столе"; GroupDescription: "Дополнительно:"; Flags: checkedonce
Name: "autorun"; Description: "Запускать вместе с Windows"; GroupDescription: "Дополнительно:"; Flags: checkedonce

[InstallDelete]
; остатки прежних версий (Zapret GUI, Deliverance)
Type: filesandordirs; Name: "{autopf}\Zapret GUI"
Type: filesandordirs; Name: "{autopf}\Deliverance"
Type: filesandordirs; Name: "{autoprograms}\Zapret GUI"
Type: filesandordirs; Name: "{autoprograms}\Deliverance"
Type: files; Name: "{autodesktop}\Zapret GUI.lnk"
Type: files; Name: "{autodesktop}\Deliverance.lnk"

[Files]
Source: "..\build\bin\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\licenses\*"; DestDir: "{app}\licenses"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon

[Run]
Filename: "schtasks"; Parameters: "/Create /TN ""DZapret"" /TR ""\""{app}\{#AppExe}\"" --minimized"" /SC ONLOGON /RL HIGHEST /F"; Flags: runhidden; Tasks: autorun
Filename: "{app}\{#AppExe}"; Description: "Запустить {#AppName}"; Flags: nowait postinstall skipifsilent
; тихое обновление из программы: после него приложение запускается само (в обычной установке — чекбокс выше)
Filename: "{app}\{#AppExe}"; Flags: nowait skipifnotsilent

[UninstallRun]
Filename: "taskkill"; Parameters: "/F /IM winws.exe"; Flags: runhidden; RunOnceId: "KillWinws"
Filename: "sc"; Parameters: "stop WinDivert"; Flags: runhidden; RunOnceId: "StopDivert"
Filename: "schtasks"; Parameters: "/Delete /TN ""DZapret"" /F"; Flags: runhidden; RunOnceId: "DelTask"

[UninstallDelete]
Type: filesandordirs; Name: "{commonappdata}\DZapret"

[Code]
// Перед установкой закрываем прежние версии и обход, чтобы файлы можно было заменить,
// и убираем задачи автозапуска под старыми названиями (новая создаётся заново, если выбрана).
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  R: Integer;
begin
  Exec('taskkill', '/F /IM dzapret.exe', '', SW_HIDE, ewWaitUntilTerminated, R);
  Exec('taskkill', '/F /IM deliverance.exe', '', SW_HIDE, ewWaitUntilTerminated, R);
  Exec('taskkill', '/F /IM zapretgui.exe', '', SW_HIDE, ewWaitUntilTerminated, R);
  Exec('taskkill', '/F /IM winws.exe', '', SW_HIDE, ewWaitUntilTerminated, R);
  Exec('schtasks', '/Delete /TN "Deliverance" /F', '', SW_HIDE, ewWaitUntilTerminated, R);
  Exec('schtasks', '/Delete /TN "ZapretGUI" /F', '', SW_HIDE, ewWaitUntilTerminated, R);
  Result := '';
end;
