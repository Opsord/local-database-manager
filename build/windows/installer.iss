#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif

#ifndef NumericVersion
  #define NumericVersion "0.0.0.0"
#endif

#ifndef SourceDir
  #define SourceDir "..\..\dist-installer"
#endif

#ifndef OutputDir
  #define OutputDir "..\..\dist-installer-out"
#endif

#ifndef OutputBaseFilename
  #define OutputBaseFilename "db-manager_Windows_x86_64-setup"
#endif

[Setup]
AppId={{8F6A2E4B-5D3C-4B89-9C1E-7A4D3E2F109B}
AppName=Local Database Manager
AppVersion={#AppVersion}
AppVerName=Local Database Manager {#AppVersion}
AppPublisher=Opsord
AppPublisherURL=https://github.com/Opsord/local-database-manager
AppSupportURL=https://github.com/Opsord/local-database-manager/issues
AppUpdatesURL=https://github.com/Opsord/local-database-manager/releases
VersionInfoVersion={#NumericVersion}
VersionInfoDescription=Local Database Manager Installer
VersionInfoProductName=Local Database Manager
DefaultDirName={localappdata}\Programs\Local Database Manager
DefaultGroupName=Local Database Manager
DisableProgramGroupPage=yes
LicenseFile=..\..\LICENSE
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
ChangesEnvironment=yes
CloseApplications=yes
CloseApplicationsFilter=*.exe
RestartApplications=no
UninstallDisplayName=Local Database Manager
OutputDir={#OutputDir}
OutputBaseFilename={#OutputBaseFilename}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Types]
Name: "full"; Description: "Full installation (Desktop GUI + CLI & TUI)"
Name: "desktop"; Description: "Desktop GUI only"
Name: "cli"; Description: "CLI & TUI only"
Name: "custom"; Description: "Custom installation"; Flags: iscustom

[Components]
Name: "desktop"; Description: "Desktop Application (db-manager-desktop.exe)"; Types: full desktop custom
Name: "cli"; Description: "CLI & Interactive TUI (db-manager.exe)"; Types: full cli custom

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Components: desktop; Flags: unchecked
Name: "addtopath"; Description: "Add installation directory to user PATH (run 'db-manager' from any terminal)"; GroupDescription: "Terminal integration:"; Components: cli

[Files]
Source: "{#SourceDir}\db-manager-desktop.exe"; DestDir: "{app}"; Components: desktop; Flags: ignoreversion
Source: "{#SourceDir}\db-manager.exe"; DestDir: "{app}"; Components: cli; Flags: ignoreversion
Source: "{#SourceDir}\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Local Database Manager"; Filename: "{app}\db-manager-desktop.exe"; Components: desktop
Name: "{autodesktop}\Local Database Manager"; Filename: "{app}\db-manager-desktop.exe"; Components: desktop; Tasks: desktopicon
Name: "{group}\Local Database Manager (TUI)"; Filename: "{app}\db-manager.exe"; Components: cli
Name: "{group}\{cm:UninstallProgram,Local Database Manager}"; Filename: "{uninstallexe}"

[Run]
Filename: "{app}\db-manager-desktop.exe"; Description: "{cm:LaunchProgram,Local Database Manager}"; Components: desktop; Flags: nowait postinstall skipifsilent

[Code]
function NormalizePathEntry(const Value: string): string;
begin
  Result := Uppercase(RemoveBackslashUnlessRoot(Trim(Value)));
end;

function PathContainsDir(const PathValue, Dir: string): Boolean;
var
  Remaining, Entry, Target: string;
  SepPos: Integer;
begin
  Result := False;
  Target := NormalizePathEntry(Dir);
  if Target = '' then
    Exit;

  Remaining := PathValue;
  while Remaining <> '' do
  begin
    SepPos := Pos(';', Remaining);
    if SepPos > 0 then
    begin
      Entry := Copy(Remaining, 1, SepPos - 1);
      Delete(Remaining, 1, SepPos);
    end
    else
    begin
      Entry := Remaining;
      Remaining := '';
    end;

    if NormalizePathEntry(Entry) = Target then
    begin
      Result := True;
      Exit;
    end;
  end;
end;

procedure AddDirToUserPath(const Dir: string);
var
  OrigPath, NewPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    RegWriteExpandStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Dir);
    Exit;
  end;

  if PathContainsDir(OrigPath, Dir) then
    Exit;

  NewPath := Trim(OrigPath);
  if NewPath = '' then
    NewPath := Dir
  else if NewPath[Length(NewPath)] = ';' then
    NewPath := NewPath + Dir
  else
    NewPath := NewPath + ';' + Dir;

  RegWriteExpandStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', NewPath);
end;

procedure RemoveDirFromUserPath(const Dir: string);
var
  OrigPath, Remaining, Entry, NewPath, Target: string;
  SepPos: Integer;
  Changed: Boolean;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
    Exit;

  Target := NormalizePathEntry(Dir);
  if Target = '' then
    Exit;

  Remaining := OrigPath;
  NewPath := '';
  Changed := False;

  while Remaining <> '' do
  begin
    SepPos := Pos(';', Remaining);
    if SepPos > 0 then
    begin
      Entry := Copy(Remaining, 1, SepPos - 1);
      Delete(Remaining, 1, SepPos);
    end
    else
    begin
      Entry := Remaining;
      Remaining := '';
    end;

    if Trim(Entry) <> '' then
    begin
      if NormalizePathEntry(Entry) = Target then
        Changed := True
      else
      begin
        if NewPath = '' then
          NewPath := Entry
        else
          NewPath := NewPath + ';' + Entry;
      end;
    end;
  end;

  if Changed then
    RegWriteExpandStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', NewPath);
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = wpSelectComponents then
  begin
    if (not WizardIsComponentSelected('desktop')) and (not WizardIsComponentSelected('cli')) then
    begin
      if not WizardSilent then
        MsgBox('Please select at least one component to install (Desktop Application or CLI & Interactive TUI).', mbError, MB_OK);
      Result := False;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    if WizardIsComponentSelected('cli') and WizardIsTaskSelected('addtopath') then
      AddDirToUserPath(ExpandConstant('{app}'))
    else
      RemoveDirFromUserPath(ExpandConstant('{app}'));
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    RemoveDirFromUserPath(ExpandConstant('{app}'));
end;
