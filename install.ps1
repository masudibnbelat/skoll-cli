# skoll installer for Windows (PowerShell).
# Usage — paste this one line in PowerShell:
#   irm https://raw.githubusercontent.com/masudibnbelat/skoll-cli/main/install.ps1 | iex
#
# What it does:
#   1. Finds the latest GitHub Release of masudibnbelat/skoll-cli.
#   2. Downloads skoll-windows-amd64.exe.
#   3. Installs it to $env:LOCALAPPDATA\skoll\bin\skoll.exe.
#   4. Adds that folder to the user PATH (new terminals pick it up).
$ErrorActionPreference = "Stop"
$Repo = "masudibnbelat/skoll-cli"
$InstallDir = Join-Path $env:LOCALAPPDATA "skoll\bin"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Write-Host "== skoll: finding latest release of $Repo"
# The GitHub web redirect gives the latest tag with no API call, so
# it never hits the unauthenticated API rate limit (60/hour per IP —
# that is what a 403 from api.github.com means). Fall back to the
# API only if the redirect fails; GITHUB_TOKEN is honoured when set.
$Tag = $null
try {
  $FinalUrl = curl.exe -fsSL -o NUL -w "%{url_effective}" "https://github.com/$Repo/releases/latest" 2>$null
  if ($FinalUrl -match "/tag/(.+)$") { $Tag = $Matches[1] }
} catch { }
if (-not $Tag) {
  try {
    $Headers = @{}
    if ($env:GITHUB_TOKEN) { $Headers.Authorization = "Bearer $env:GITHUB_TOKEN" }
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers $Headers
    $Tag = $Release.tag_name
  } catch { }
}
if (-not $Tag) { throw "Could not find the latest release. If GitHub answered 403, the API rate limit (60/hour per IP) is hit — wait an hour, or set GITHUB_TOKEN and retry. If no release exists yet, publish one: git tag v0.6.0; git push origin v0.6.0" }
Write-Host "== latest: $Tag — downloading skoll-windows-amd64.exe"
$Url = "https://github.com/$Repo/releases/download/$Tag/skoll-windows-amd64.exe"
Invoke-WebRequest -Uri $Url -OutFile (Join-Path $InstallDir "skoll.exe")

# Add to the user PATH (persists; new terminals get it).
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
}
$env:Path = "$env:Path;$InstallDir"

Write-Host "== DONE: skoll $Tag installed to $InstallDir\skoll.exe"
& skoll --version
Write-Host "Close and reopen the terminal, then in any Android project:"
Write-Host "  cd MyApp"
Write-Host "  skoll run"
Write-Host '  skoll release v1.5.0 "notes"'
Write-Host "Note: Android SDK, adb, Java and gh still need to be installed once"
Write-Host "on Windows (Android Studio installs SDK + adb; `gh auth login` once)."
