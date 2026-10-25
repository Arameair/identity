# Build Identity Prototype B on Windows from this checkout (embeds the commit SHA).
# Usage (PowerShell, from the repository folder):  .\scripts\build-windows.ps1
$ErrorActionPreference = "Stop"
go version
go vet ./...
go test ./...
go build -ldflags "-X main.buildBranch=experiment/identity-home-claude-20261025" -o identity-b.exe .
Write-Host ""
Write-Host "Built identity-b.exe from commit $(git rev-parse HEAD)"
Write-Host "Start it with:  .\identity-b.exe"
