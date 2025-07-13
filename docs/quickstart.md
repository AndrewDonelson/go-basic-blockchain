# Quick Start Guide

Get up and running with Go Basic Blockchain in minutes! This guide will help you set up the project, run your first blockchain node, and start exploring the system.

## 🚀 Prerequisites

### Required Software
- **Go 1.19+**: [Download Go](https://golang.org/dl/)
- **Git**: [Download Git](https://git-scm.com/)
- **Make**: Usually pre-installed on Linux/macOS, [Windows instructions](https://chocolatey.org/packages/make)

### System Requirements
- **RAM**: 512MB minimum, 2GB recommended
- **Storage**: 100MB free space
- **Network**: Internet connection for dependencies

## 📦 Installation

### 1. Clone the Repository
```bash
git clone https://github.com/yourusername/go-basic-blockchain.git
cd go-basic-blockchain
```

### 2. Install Dependencies
```bash
go mod tidy
```

### 3. Verify Installation
```bash
make test
```

You should see output similar to:
```
=== RUN   TestWallet
--- PASS: TestWallet (0.12s)
=== RUN   TestAPI
--- PASS: TestAPI (0.05s)
...
PASS
ok      github.com/yourusername/go-basic-blockchain/sdk 9.5s
```

## 🏃‍♂️ First Run

### 1. Start the Blockchain Node
```bash
make run
```

You should see output like:
```
Starting blockchain node...
API server running on :8200
Blockchain initialized with genesis block
```

### 2. Access the Web Interface
Open your browser and navigate to:
```
http://localhost:8200
```

You should see the blockchain web interface with:
- Current block height
- Recent transactions
- Network status
- Mining controls

### 3. Create Your First Wallet
Using the web interface or API:

**Via Web Interface:**
1. Click "Create Wallet"
2. Enter a strong passphrase
3. Save your wallet file securely

**Via API:**
```bash
curl -X POST http://localhost:8200/api/wallet/create \
  -H "Content-Type: application/json" \
  -d '{"passphrase": "your-strong-passphrase"}'
```

### 4. Start Mining
**Via Web Interface:**
1. Click "Start Mining"
2. Watch blocks being created
3. Check your wallet balance

**Via API:**
```bash
curl -X POST http://localhost:8200/api/mining/start
```

## 🔧 Basic Operations

### Creating Transactions

**Bank Transaction (Transfer coins):**
```bash
curl -X POST http://localhost:8200/api/transaction/create \
  -H "Content-Type: application/json" \
  -d '{
    "type": "BANK",
    "from": "wallet-address",
    "to": "recipient-address", 
    "amount": 10.5,
    "passphrase": "your-passphrase"
  }'
```

**Message Transaction (Send encrypted message):**
```bash
curl -X POST http://localhost:8200/api/transaction/create \
  -H "Content-Type: application/json" \
  -d '{
    "type": "MESSAGE",
    "from": "wallet-address",
    "to": "recipient-address",
    "message": "Hello, blockchain!",
    "passphrase": "your-passphrase"
  }'
```

### Checking Blockchain Status

**Get current block height:**
```bash
curl http://localhost:8200/api/blockchain/status
```

**Get recent blocks:**
```bash
curl http://localhost:8200/api/blockchain/blocks
```

**Get wallet balance:**
```bash
curl http://localhost:8200/api/wallet/balance/wallet-address
```

## 🧪 Testing Features

### Run All Tests
```bash
make test
```

### Run Specific Test Categories
```bash
# Wallet tests only
go test ./sdk -run TestWallet

# API tests only  
go test ./sdk -run TestAPI

# Blockchain tests only
go test ./sdk -run TestBlockchain
```

### Performance Testing
```bash
# Run tests with performance profiling
go test ./sdk -bench=.

# Run tests with coverage
go test ./sdk -cover
```

## 🔍 Exploring the System

### 1. Block Explorer
Visit `http://localhost:8200/explorer` to:
- Browse all blocks
- View transaction details
- Search by block hash or transaction ID
- Monitor network activity

### 2. API Documentation
The API is self-documenting. Visit:
- `http://localhost:8200/api/` - API root
- `http://localhost:8200/api/docs` - Interactive documentation

### 3. Logs and Debugging
Check the console output for:
- Block creation events
- Transaction processing
- Network activity
- Error messages

## 🛠️ Development Setup

### 1. IDE Configuration
Recommended VS Code extensions:
- Go extension
- GitLens
- REST Client

### 2. Debug Configuration
Create `.vscode/launch.json`:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Blockchain",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/chaind/main.go"
    }
  ]
}
```

### 3. Hot Reload (Optional)
Install `air` for hot reloading:
```bash
go install github.com/cosmtrek/air@latest
air
```

## 🚨 Troubleshooting

### Common Issues

**Port Already in Use:**
```bash
# Find process using port 8200
lsof -i :8200
# Kill the process
kill -9 <PID>
```

**Permission Denied:**
```bash
# Make sure you have write permissions
chmod +x bin/chaind
```

**Go Module Issues:**
```bash
# Clean and reinstall
go clean -modcache
go mod tidy
```

**Test Failures:**
```bash
# Run with verbose output
go test ./sdk -v

# Run with timeout
go test ./sdk -timeout 60s
```

### Getting Help

1. **Check the logs**: Look for error messages in console output
2. **Verify prerequisites**: Ensure Go 1.19+ is installed
3. **Check network**: Ensure port 8200 is available
4. **Review documentation**: See [Troubleshooting](troubleshooting.md) for more details

## 📚 Next Steps

Now that you're up and running:

1. **Explore the Architecture**: Read [Architecture](architecture.md) to understand the system design
2. **Learn About Helios**: Study [Helios Consensus](helios.md) for advanced features
3. **Master Wallets**: Follow [Wallet Guide](wallet.md) for security best practices
4. **API Integration**: Use [API Reference](api.md) for programmatic access
5. **Contribute**: Check [Development Guide](development.md) for contributing guidelines

---

**Congratulations! You're now running your own blockchain node. Explore the system and start building!** 