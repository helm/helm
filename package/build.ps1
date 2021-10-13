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

$binary_name = "helm-" + $ci_version + "-windows-amd64"

Write-Host "App Version :" $ci_version "Binary Name :" $binary_name

$process = Start-Process -FilePath ${Env:ProgramFiles(x86)}"\Inno Setup 6\ISCC.exe" -ArgumentList "helm_installer.iss", "/Dversion=$ci_version", "/Dopname=$binary_name" -NoNewWindow -PassThru -Wait

$process.WaitForExit()

Write-Host "Inno Compiler exit code : " $process.ExitCode


$output = "Output\"
$binary_path = $output + $binary_name + ".exe"

$hash = Get-FileHash -Path $binary_path -Algorithm SHA256

$opsha = $output + [System.IO.Path]::GetFileName($hash.Path) + $hash.Algorithm.ToLower()
$hash.Hash > $opsha

$opshasum = $opsha + "sum"

$hash.Hash + "  " + $binary_name + ".exe" > $opshasum