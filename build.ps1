$ErrorActionPreference = "Stop"

$projectRoot = $PSScriptRoot
$outputDirectory = Join-Path $projectRoot "bin"
$outputPath = Join-Path $outputDirectory "regieleki.exe"

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

& go build `
    -trimpath `
    -buildvcs=false `
    -ldflags="-s -w" `
    -o $outputPath `
    $projectRoot

if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}

Write-Host "Built $outputPath"
