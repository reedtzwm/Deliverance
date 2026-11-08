; Inno Setup — один Setup.exe: ставит программу, ярлыки, запись об удалении.
; Файлы zapret приложение скачивает само в %ProgramData%\Deliverance.
#define AppName "Deliverance"
#ifndef AppVer
  #define AppVer "1.7.1"
#endif
#define AppExe  "deliverance.exe"

[Setup]
; AppId прежний — новая версия встаёт поверх установленной (до переименования программа называлась Zapret GUI)
AppId={{B7E2C4A1-5D3F-4E8A-9C61-0A1B2C3D4E5F}
AppName={#AppName}
AppVersion={#AppVer}
AppPublisher=Deliverance
DefaultDirName={autopf}\{#AppName}
UsePreviousAppDir=no
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\build\installer
OutputBaseFilename=Deliverance-Setup-{#AppVer}
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
Name: "desktopicon"; Description: "Создать ярлык на рабочем столе"; GroupDescription: "Дополнительно:"
Name: "autorun"; Description: "Запускать вместе с Windows"; GroupDescription: "Дополнительно:"

[InstallDelete]
; остатки прежней версии (Zapret GUI)
Type: filesandordirs; Name: "{autopf}\Zapret GUI"
Type: filesandordirs; Name: "{autoprograms}\Zapret GUI"
Type: files; Name: "{autodesktop}\Zapret GUI.lnk"

[Files]
Source: "..\build\bin\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\licenses\*"; DestDir: "{app}\licenses"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon

[Run]
Filename: "schtasks"; Parameters: "/Create /TN ""Deliverance"" /TR ""\""{app}\{#AppExe}\"" --minimized"" /SC ONLOGON /RL HIGHEST /F"; Flags: runhidden; Tasks: autorun
Filename: "{app}\{#AppExe}"; Description: "Запустить {#AppName}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "taskkill"; Parameters: "/F /IM winws.exe"; Flags: runhidden; RunOnceId: "KillWinws"
Filename: "sc"; Parameters: "stop WinDivert"; Flags: runhidden; RunOnceId: "StopDivert"
Filename: "schtasks"; Parameters: "/Delete /TN ""Deliverance"" /F"; Flags: runhidden; RunOnceId: "DelTask"
Filename: "schtasks"; Parameters: "/Delete /TN ""ZapretGUI"" /F"; Flags: runhidden; RunOnceId: "DelOldTask"

[UninstallDelete]
Type: filesandordirs; Name: "{commonappdata}\Deliverance"
Type: filesandordirs; Name: "{commonappdata}\ZapretGUI"

[Code]
// Перед установкой закрываем прежнюю версию (Zapret GUI) и обход, чтобы файлы можно было заменить.
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  R: Integer;
begin
  Exec('taskkill', '/F /IM zapretgui.exe', '', SW_HIDE, ewWaitUntilTerminated, R);
  Exec('taskkill', '/F /IM winws.exe', '', SW_HIDE, ewWaitUntilTerminated, R);
  Exec('schtasks', '/Delete /TN "ZapretGUI" /F', '', SW_HIDE, ewWaitUntilTerminated, R);
  Result := '';
end;
