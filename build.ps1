param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "dev"
    $branch = (& git branch --show-current 2>$null).Trim()
    $tag = (& git describe --tags --exact-match HEAD 2>$null | Select-Object -First 1).Trim()
    & git diff --quiet
    $workingTreeClean = $LASTEXITCODE -eq 0
    & git diff --cached --quiet
    $workingTreeClean = $workingTreeClean -and ($LASTEXITCODE -eq 0)

    if ($workingTreeClean -and ($branch -eq "main" -or [string]::IsNullOrEmpty($branch)) -and $tag -match "^v(.+)$") {
        $Version = $Matches[1]
    }
}

if ($Version.StartsWith("v")) {
    $Version = $Version.Substring(1)
}

$projectRoot = $PSScriptRoot
$outputDirectory = Join-Path $projectRoot "bin"
$outputPath = Join-Path $outputDirectory "regieleki.exe"

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

& go build `
    -trimpath `
    -buildvcs=false `
    -ldflags="-s -w -X main.version=$Version" `
    -o $outputPath `
    $projectRoot

if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}

Write-Host "Built $outputPath (version $Version)"
