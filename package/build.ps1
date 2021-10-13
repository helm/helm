if (![string]::IsNullOrEmpty(${Env:CIRCLE_PR_NUMBER})) {
    Write-Host "Skipping deploy step; as this is a PR"
    exit;
}

if (![string]::IsNullOrEmpty(${Env:CIRCLE_TAG})) {
    $ci_version = ${Env:CIRCLE_TAG}
}
elseif ( ${Env:CIRCLE_BRANCH} -eq "main" ) {
    $ci_version = "canary"
}
else {
    Write-Host "Skipping deploy step; this is neither a releasable branch or a tag"
    exit;
}

Invoke-WebRequest -Uri https://jrsoftware.org/download.php/is.exe -OutFile inno.exe

$process = Start-Process -FilePath .\inno.exe -ArgumentList "/VERYSILENT", "/NORESTART" -NoNewWindow -PassThru -Wait

$process.WaitForExit()

Write-Host "Inno installer exit code : " $process.ExitCode

Write-Host "App Version : " $ci_version

$process = Start-Process -FilePath ${Env:ProgramFiles(x86)}"\Inno Setup 6\ISCC.exe" -ArgumentList "helm_installer.iss", "/DVersion=$ci_version" -NoNewWindow -PassThru -Wait

$process.WaitForExit()

Write-Host "Inno Compiler exit code : " $process.ExitCode