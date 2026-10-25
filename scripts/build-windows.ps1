# Build Identity on Windows from this checkout (embeds commit and branch).
# Usage (PowerShell, from the repository folder):  .\scripts\build-windows.ps1
$ErrorActionPreference = "Stop"
go version
go vet ./...
go test ./...
$branch = (git rev-parse --abbrev-ref HEAD).Trim()
go build -ldflags "-X main.buildBranch=$branch" -o identity.exe .
Write-Host ""
Write-Host "Built identity.exe from $branch @ $(git rev-parse HEAD)"
Write-Host "Start it with:  .\identity.exe"
