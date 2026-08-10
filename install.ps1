# 一键安装 cursor-mode-model（Windows）。
# 用法：irm https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.ps1 | iex
[CmdletBinding()]
param(
    [string]$Version,
    [switch]$SkipInstallHooks
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Step {
    param([string]$Message)
    Write-Host "cursor-mode-model: $Message"
}

function Get-RequestHeaders {
    $headers = @{
        Accept = "application/vnd.github+json"
        "User-Agent" = "cursor-mode-model-installer"
        "X-GitHub-Api-Version" = "2022-11-28"
    }
    $token = if ($env:GH_TOKEN) { $env:GH_TOKEN } elseif ($env:GITHUB_TOKEN) { $env:GITHUB_TOKEN } else { $null }
    if ($token) {
        $headers.Authorization = "Bearer $token"
    }
    return $headers
}

function Add-ToUserPath {
    param([string]$Directory)
    $trimCharacters = [char[]]"\/"
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @()
    if ($userPath) {
        $entries = @($userPath -split ";" | Where-Object { $_ })
    }
    $normalizedDirectory = $Directory.TrimEnd($trimCharacters)
    $alreadyPresent = $entries | Where-Object {
        $_.Trim().TrimEnd($trimCharacters).Equals($normalizedDirectory, [StringComparison]::OrdinalIgnoreCase)
    }
    if (-not $alreadyPresent) {
        $newPath = (@($entries) + $Directory) -join ";"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Step "Added $Directory to your user PATH. Open a new terminal to use it."
    }
    $processEntries = @($env:Path -split ";" | Where-Object { $_ })
    $processPresent = $processEntries | Where-Object {
        $_.Trim().TrimEnd($trimCharacters).Equals($normalizedDirectory, [StringComparison]::OrdinalIgnoreCase)
    }
    if (-not $processPresent) {
        $env:Path = (@($processEntries) + $Directory) -join ";"
    }
}

try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    if (-not $Version -and $env:CMM_VERSION) {
        $Version = $env:CMM_VERSION
    }
    if ($Version -and $Version -notmatch '^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$') {
        throw "Invalid version '$Version'. Expected a value such as 0.2.0 or v0.2.0."
    }

    $repository = if ($env:CMM_REPO) { $env:CMM_REPO } else { "x0c/cursor-mode-model" }
    $headers = Get-RequestHeaders
    $apiBase = "https://api.github.com/repos/$repository/releases"

    if ($Version) {
        $tag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
        Write-Step "Resolving release $tag..."
        $release = Invoke-RestMethod -Uri "$apiBase/tags/$([Uri]::EscapeDataString($tag))" -Headers $headers
    }
    else {
        Write-Step "Resolving the latest release..."
        $release = Invoke-RestMethod -Uri "$apiBase/latest" -Headers $headers
        $tag = [string]$release.tag_name
    }

    $versionNumber = $tag.TrimStart("v")
    $arch = if ([Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { "arm64" } else { "amd64" }
    } else {
        throw "32-bit Windows is not supported."
    }
    $assetName = "cursor-mode-model_${versionNumber}_windows_${arch}.zip"
    $asset = @($release.assets | Where-Object { $_.name -eq $assetName }) | Select-Object -First 1
    if (-not $asset) {
        throw "Release $tag does not contain $assetName. Fallback: go install github.com/x0c/cursor-mode-model/cmd/cursor-mode-model@$tag"
    }

    $installDirectory = if ($env:CMM_PREFIX) {
        Join-Path ([IO.Path]::GetFullPath($env:CMM_PREFIX)) "bin"
    } else {
        Join-Path $env:USERPROFILE ".local\bin"
    }
    New-Item -ItemType Directory -Force -Path $installDirectory | Out-Null

    $tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("cursor-mode-model-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
    try {
        $zipPath = Join-Path $tempRoot $assetName
        Write-Step "Downloading $assetName..."
        Invoke-WebRequest -Uri $asset.browser_download_url -Headers $headers -OutFile $zipPath
        Expand-Archive -Path $zipPath -DestinationPath $tempRoot -Force
        $exe = Get-ChildItem -Path $tempRoot -Filter "cursor-mode-model.exe" -Recurse | Select-Object -First 1
        if (-not $exe) {
            throw "Archive did not contain cursor-mode-model.exe"
        }
        $target = Join-Path $installDirectory "cursor-mode-model.exe"
        Copy-Item -Path $exe.FullName -Destination $target -Force
        Add-ToUserPath -Directory $installDirectory
        Write-Step "Installed $target"

        if (-not $SkipInstallHooks) {
            & $target install
        }
    }
    finally {
        Remove-Item -Recurse -Force $tempRoot -ErrorAction SilentlyContinue
    }

    Write-Step "Done. Run: cursor-mode-model status"
    Write-Step "Prerequisite: Cursor Agent CLI installed and logged in (https://cursor.com/install)."
}
catch {
    Write-Error $_
    exit 1
}
