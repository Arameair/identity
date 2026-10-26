# Build Identity and its test client on Windows from this checkout (embeds commit and branch).
# Usage (PowerShell, from the repository folder):  .\scripts\build-windows.ps1
$ErrorActionPreference = "Stop"
go version
go vet ./...
if ($LASTEXITCODE -ne 0) { Write-Host "go vet FAILED - stopping"; exit 1 }
go test ./...
if ($LASTEXITCODE -ne 0) { Write-Host "go test FAILED - stopping"; exit 1 }
$branch = (git rev-parse --abbrev-ref HEAD).Trim()
go build -ldflags "-X main.buildBranch=$branch" -o identity.exe .
if ($LASTEXITCODE -ne 0) { exit 1 }
go build -o identity-probe.exe ./cmd/identity-probe
if ($LASTEXITCODE -ne 0) { exit 1 }
Write-Host ""
Write-Host "Built identity.exe and identity-probe.exe from $branch @ $(git rev-parse HEAD)"
Write-Host "Self-test:   .\identity-probe.exe smoke"
Write-Host "Start it:    .\identity.exe"
