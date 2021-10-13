[Setup]
AppName=Helm
AppVersion={#Version}
DefaultDirName={autopf}\Helm
DefaultGroupName=Helm
PrivilegesRequired=lowest
AppPublisher=Helm
AppPublisherURL=https://helm.sh
AppSupportURL=https://github.com/helm/helm
LicenseFile="windows-amd64\LICENSE"
OutputBaseFilename=helm_installer_win64

[Files]
Source: "windows-amd64\*" ; DestDir: "{app}\bin";

[Registry]
Root: "HKCU"; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}\bin"; Check: NeedsAddPathHKCU(ExpandConstant('{app}\bin'))

[Code]
function NeedsAddPathHKCU(Param: string): boolean;
var
OrigPath: string;
begin
if not RegQueryStringValue(HKEY_CURRENT_USER,
'Environment',
'Path', OrigPath)
then begin
Result := True;
exit;
end;
// look for the path with leading and trailing semicolon
// Pos() returns 0 if not found
Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;