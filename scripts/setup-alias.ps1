[CmdletBinding()]
param (
    [Parameter(Mandatory = $false)]
    [string]$RepoPath = "",

    [Parameter(Mandatory = $false)]
    [string]$TargetProfile = "",

    [Parameter(Mandatory = $false)]
    [switch]$BuildIfMissing = $true
)

# 1. Resolve repository path dynamically
if ([string]::IsNullOrWhiteSpace($RepoPath)) {
    if ($PSScriptRoot) {
        $RepoPath = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
    } else {
        $RepoPath = (Get-Location).Path
    }
} else {
    $RepoPath = (Resolve-Path $RepoPath).Path
}

# Verify that this is the local-database-manager repository
$hasEngines = Test-Path (Join-Path $RepoPath "engines")
$hasInstances = Test-Path (Join-Path $RepoPath "instances")
if (-not ($hasEngines -and $hasInstances)) {
    Write-Error "The directory '$RepoPath' does not appear to be a valid local-database-manager repository (missing engines/ or instances/)."
    return
}

Write-Host "Using repository path: $RepoPath" -ForegroundColor Cyan

# 2. Check / Build executable binary
$exePath = Join-Path $RepoPath "db-manager.exe"
if (-not (Test-Path $exePath)) {
    if ($BuildIfMissing) {
        Write-Host "db-manager.exe not found in $RepoPath. Building with Go..." -ForegroundColor Cyan
        Push-Location $RepoPath
        try {
            go build -o db-manager.exe ./cmd/db-manager
            if ($LASTEXITCODE -ne 0) {
                Write-Error "Failed to build db-manager.exe. Please ensure Go is installed and build manually."
                return
            }
            Write-Host "Successfully compiled db-manager.exe!" -ForegroundColor Green
        } finally {
            Pop-Location
        }
    } else {
        Write-Warning "db-manager.exe not found. Remember to compile it with: go build -o db-manager.exe ./cmd/db-manager"
    }
} else {
    Write-Host "Found compiled binary: $exePath" -ForegroundColor Green
}

# 3. Determine target profile
if ([string]::IsNullOrWhiteSpace($TargetProfile)) {
    $TargetProfile = $PROFILE
}

$profileDir = Split-Path -Parent $TargetProfile
if (-not (Test-Path $profileDir)) {
    New-Item -ItemType Directory -Path $profileDir -Force | Out-Null
}
if (-not (Test-Path $TargetProfile)) {
    New-Item -ItemType File -Path $TargetProfile -Force | Out-Null
}

# 4. Construct function block with idempotent markers
$markerStart = "# >>> local-database-manager (dbm) >>>"
$markerEnd   = "# <<< local-database-manager (dbm) <<<"

$block = @"
$markerStart
function dbm {
    Push-Location "$RepoPath"
    try {
        .\db-manager.exe `$args
    } finally {
        Pop-Location
    }
}
$markerEnd
"@

# 5. Idempotently insert or update profile
$content = Get-Content -Path $TargetProfile -Raw -ErrorAction SilentlyContinue
if ($null -eq $content) { $content = "" }

$pattern = "(?s)" + [regex]::Escape($markerStart) + ".*?" + [regex]::Escape($markerEnd)
if ($content -match $pattern) {
    $content = $content -replace $pattern, $block
    Set-Content -Path $TargetProfile -Value $content -NoNewline
    Write-Host "Updated existing 'dbm' function in $TargetProfile" -ForegroundColor Green
} else {
    Add-Content -Path $TargetProfile -Value "`r`n$block"
    Write-Host "Added 'dbm' shortcut function to $TargetProfile" -ForegroundColor Green
}

# 6. Load into current session
try {
    . $TargetProfile
    Write-Host "Profile reloaded successfully. You can now run 'dbm' from anywhere!" -ForegroundColor Green
} catch {
    # If script execution is restricted, define function in current session as fallback
    Invoke-Expression "function dbm { Push-Location `"$RepoPath`"; try { .\db-manager.exe `$args } finally { Pop-Location } }"
    Write-Warning "Could not dot-source $TargetProfile due to execution policy restriction. Function 'dbm' was registered for the active session."
    Write-Host "To allow PowerShell profiles permanently, run: Set-ExecutionPolicy RemoteSigned -Scope CurrentUser" -ForegroundColor Yellow
}
