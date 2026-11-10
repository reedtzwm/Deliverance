# Release build: app -> (optional Authenticode) -> installer -> (optional Authenticode) -> update signature.
#
# Optional environment variables:
#   SIGN_PFX / SIGN_PFX_PASS   code-signing certificate (.pfx) for Windows Authenticode (removes SmartScreen warnings)
#   UPDATE_KEY                 path to update signing key (default: %USERPROFILE%\.zapretgui-signing\update.key)
param([switch]$SkipUpdateSig)
$ErrorActionPreference = 'Stop'
$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')
Set-Location $PSScriptRoot

function Sign-Authenticode($file) {
    if (-not $env:SIGN_PFX) { return }
    $st = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue | Sort-Object FullName | Select-Object -Last 1
    if (-not $st) { throw 'signtool.exe not found (install Windows SDK)' }
    & $st.FullName sign /f $env:SIGN_PFX /p $env:SIGN_PFX_PASS /fd sha256 /tr http://timestamp.digicert.com /td sha256 $file
    if ($LASTEXITCODE) { throw "signtool failed for $file" }
}

# сначала сборка: wails собирает фронтенд в frontend\dist, который нужен go:embed для тестов
& "$(go env GOPATH)\bin\wails.exe" build -clean
if ($LASTEXITCODE) { throw 'wails build failed' }
# wails build -clean стирает frontend\dist вместе с .gitkeep, а он нужен репозиторию для go:embed
New-Item -ItemType File -Force frontend\dist\.gitkeep | Out-Null

go test ./...
if ($LASTEXITCODE) { throw 'tests failed' }
Sign-Authenticode 'build\bin\dzapret.exe'

$ver = (Select-String -Path selfupdate.go -Pattern 'appVersion = "([^"]+)"').Matches[0].Groups[1].Value
& "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe" /DAppVer=$ver installer\setup.iss
if ($LASTEXITCODE) { throw 'ISCC failed' }
$setup = "build\installer\DZapret-Setup-$ver.exe"
Sign-Authenticode $setup

if (-not $SkipUpdateSig) {
    $key = if ($env:UPDATE_KEY) { $env:UPDATE_KEY } else { "$env:USERPROFILE\.zapretgui-signing\update.key" }
    if (-not (Test-Path $key)) { throw "update key not found: $key" }
    go run ./tools/sigtool sign $key $setup
    if ($LASTEXITCODE) { throw 'update signing failed' }
    Write-Host "Publish both files in the GitHub release: $setup and $setup.sig"
}

# Compatibility: copies installed before the rename (Deliverance) look for Deliverance-Setup-<ver>.exe.
# The file is the same installer under the old name; publish these two as well until old copies have updated.
$legacy = "build\installer\Deliverance-Setup-$ver.exe"
Copy-Item $setup $legacy -Force
if (Test-Path "$setup.sig") {
    Copy-Item "$setup.sig" "$legacy.sig" -Force
    Write-Host "Also publish (for old installs): $legacy and $legacy.sig"
}
