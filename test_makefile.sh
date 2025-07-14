#!/bin/bash

echo "🧪 Testing Go Basic Blockchain Makefile Targets"
echo "================================================"

# Test help
echo "📋 Testing 'make help'..."
make help
echo ""

# Test setup
echo "🔧 Testing 'make setup'..."
make setup
echo ""

# Test fmt
echo "📝 Testing 'make fmt'..."
make fmt
echo ""

# Test build
echo "🔨 Testing 'make build'..."
make build
echo ""

# Check if binary was created
if [ -f "bin/gbbd.exe" ]; then
    echo "✅ Binary created successfully: bin/gbbd.exe"
    ls -la bin/
else
    echo "❌ Binary not found"
fi
echo ""

# Test clean
echo "🧹 Testing 'make clean'..."
make clean
echo ""

# Test build again
echo "🔨 Testing 'make build' again..."
make build
echo ""

# Test demo
echo "🎬 Testing 'make demo'..."
timeout 10s make demo || echo "Demo completed or timed out"
echo ""

echo "🎉 Makefile testing completed!"
echo ""
echo "Available targets:"
make help 