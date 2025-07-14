# PowerShell script to test Makefile targets using Git Bash
Write-Host "🧪 Testing Go Basic Blockchain Makefile Targets" -ForegroundColor Green
Write-Host "================================================" -ForegroundColor Green

# Function to run make command in Git Bash
function Invoke-Make {
    param([string]$Target)
    Write-Host "📋 Testing 'make $Target'..." -ForegroundColor Yellow
    $result = bash -c "make $Target" 2>&1
    Write-Host $result
    Write-Host ""
}

# Test help
Invoke-Make "help"

# Test setup
Invoke-Make "setup"

# Test fmt
Invoke-Make "fmt"

# Test build
Invoke-Make "build"

# Check if binary was created
if (Test-Path "bin/gbbd.exe") {
    Write-Host "✅ Binary created successfully: bin/gbbd.exe" -ForegroundColor Green
    Get-ChildItem bin/
} else {
    Write-Host "❌ Binary not found" -ForegroundColor Red
}
Write-Host ""

# Test clean
Invoke-Make "clean"

# Test build again
Invoke-Make "build"

Write-Host "🎉 Makefile testing completed!" -ForegroundColor Green 