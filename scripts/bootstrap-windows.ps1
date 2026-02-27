Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host "[OK]   $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-Fail {
    param([string]$Message)
    Write-Host "[ERR]  $Message" -ForegroundColor Red
}

function Test-Command {
    param([string]$Name)
    return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
}

function Add-PathIfMissing {
    param([string]$PathToAdd)

    if (-not (Test-Path $PathToAdd)) {
        return
    }

    $parts = $env:Path -split ";"
    if ($parts -notcontains $PathToAdd) {
        $env:Path = "$env:Path;$PathToAdd"
    }

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $userParts = @()
    if ($userPath) {
        $userParts = $userPath -split ";"
    }
    if ($userParts -notcontains $PathToAdd) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $PathToAdd
        } else {
            "$userPath;$PathToAdd"
        }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    }
}

function Ensure-Node {
    Write-Step "Checking Node.js and npm..."
    if (-not (Test-Command "node")) {
        throw "Node.js is required but not found. Install Node.js 18+ first: https://nodejs.org/"
    }
    if (-not (Test-Command "npm")) {
        throw "npm is required but not found. Reinstall Node.js from https://nodejs.org/."
    }
    $nodeVersion = (& node -v).Trim()
    $npmVersion = (& npm -v).Trim()
    Write-Ok "Node.js $nodeVersion, npm $npmVersion"
}

function Ensure-Go {
    if (Test-Command "go") {
        $goVersion = (& go version).Trim()
        Write-Ok "$goVersion"
        return
    }

    Write-Warn "Go not found. Starting installer..."
    $goVersion = "1.23.7"
    $msiName = "go$goVersion.windows-amd64.msi"
    $downloadUrl = "https://go.dev/dl/$msiName"
    $tempDir = Join-Path $env:TEMP "dedao-bootstrap"
    $msiPath = Join-Path $tempDir $msiName

    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    Write-Step "Downloading $downloadUrl"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $msiPath

    Write-Step "Installing Go (UAC prompt may appear)..."
    Start-Process -FilePath "msiexec.exe" -ArgumentList "/i `"$msiPath`" /passive /norestart" -Wait

    $goBin = "C:\Program Files\Go\bin"
    Add-PathIfMissing $goBin

    if (-not (Test-Command "go")) {
        throw "Go installation finished but go is still unavailable. Open a new terminal and retry."
    }
    Write-Ok "$((& go version).Trim())"
}

function Ensure-Wails {
    if (Test-Command "wails") {
        Write-Ok "Wails CLI found: $((& wails version).Trim())"
        return
    }

    Write-Step "Installing Wails CLI v2.10.1..."
    & go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1

    $goPath = (& go env GOPATH).Trim()
    $goBin = Join-Path $goPath "bin"
    Add-PathIfMissing $goBin

    if (-not (Test-Command "wails")) {
        throw "Wails CLI installed but not on PATH. Add '$goBin' to PATH and retry."
    }
    Write-Ok "Wails CLI ready: $((& wails version).Trim())"
}

try {
    $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    $rootDir = Resolve-Path (Join-Path $scriptDir "..")
    Set-Location $rootDir

    Write-Step "Project root: $rootDir"
    Ensure-Node
    Ensure-Go
    Ensure-Wails

    Write-Step "Downloading Go dependencies..."
    & go mod download

    Write-Step "Installing frontend dependencies..."
    if (Test-Path "frontend/package-lock.json") {
        & npm --prefix frontend ci --no-fund --no-audit
    } else {
        & npm --prefix frontend install --no-fund --no-audit
    }

    Write-Step "Running frontend build check..."
    & npm --prefix frontend run build

    Write-Ok "Windows environment is ready."
    Write-Host ""
    Write-Host "Next step:" -ForegroundColor Cyan
    Write-Host "  wails dev"
}
catch {
    Write-Fail $_.Exception.Message
    exit 1
}
