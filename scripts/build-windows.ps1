# Build Identity and its test client on Windows from this checkout (embeds commit and branch).
# Usage (PowerShell, from the repository folder):  .\scripts\build-windows.ps1
$ErrorActionPreference = "Stop"
go version
go vet ./...
go test ./...
$branch = (git rev-parse --abbrev-ref HEAD).Trim()
go build -ldflags "-X main.buildBranch=$branch" -o identity.exe .
go build -o identity-probe.exe ./cmd/identity-probe
Write-Host ""
Write-Host "Built identity.exe and identity-probe.exe from $branch @ $(git rev-parse HEAD)"
Write-Host "Self-test:   .\identity-probe.exe smoke"
Write-Host "Start it:    .\identity.exe"
