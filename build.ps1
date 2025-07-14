# PowerShell script to build the Go Basic Blockchain project
Write-Host "🔨 Building Go Basic Blockchain (gbbd)..." -ForegroundColor Green

# Create bin directory if it doesn't exist
if (!(Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
    Write-Host "📁 Created bin directory" -ForegroundColor Yellow
}

# Build the binary
Write-Host "📦 Building binary..." -ForegroundColor Yellow
go build -o bin/gbbd.exe cmd/chaind/main.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Build successful! Binary created: bin/gbbd.exe" -ForegroundColor Green
    Get-ChildItem bin/gbbd.exe | Format-List Name, Length, LastWriteTime
} else {
    Write-Host "❌ Build failed!" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "🚀 To run the blockchain:" -ForegroundColor Cyan
Write-Host "   ./bin/gbbd.exe" -ForegroundColor White
Write-Host "" 