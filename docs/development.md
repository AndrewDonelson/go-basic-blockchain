# Development Guide

Complete guide for contributing to the Go Basic Blockchain project, including development setup, coding standards, and workflow.

## 🎯 Getting Started

### Prerequisites

**Required Software**:
- Go 1.19+ ([Download](https://golang.org/dl/))
- Git ([Download](https://git-scm.com/))
- Make (usually pre-installed)
- Your favorite IDE (VS Code recommended)

**Recommended Tools**:
- VS Code with Go extension
- GoLand (JetBrains)
- Vim/Emacs with Go plugins

### Development Setup

1. **Fork the Repository**:
   ```bash
   git clone https://github.com/yourusername/go-basic-blockchain.git
   cd go-basic-blockchain
   ```

2. **Install Dependencies**:
   ```bash
   go mod tidy
   ```

3. **Run Tests**:
   ```bash
   make test
   ```

4. **Start Development Server**:
   ```bash
   make run
   ```

## 🏗️ Project Structure

### Directory Layout

```
go-basic-blockchain/
├── cmd/                    # Application entry points
│   └── chaind/            # Main blockchain daemon
├── sdk/                   # Core blockchain implementation
│   ├── blockchain.go      # Main blockchain logic
│   ├── block.go           # Block structure and operations
│   ├── transaction.go     # Transaction types and handling
│   ├── wallet.go          # Wallet implementation
│   ├── api.go             # REST API endpoints
│   ├── p2p.go             # Peer-to-peer networking
│   ├── helios.go          # Helios consensus algorithm
│   └── ...                # Other core components
├── docs/                  # Documentation
├── scripts/               # Build and utility scripts
├── postman/               # API testing collections
├── Makefile               # Build automation
├── go.mod                 # Go module definition
└── README.md              # Project overview
```

### Key Components

**Core Blockchain** (`sdk/blockchain.go`):
- Main blockchain implementation
- Block creation and validation
- Transaction processing
- Mining operations

**API Layer** (`sdk/api.go`):
- RESTful API endpoints
- Request/response handling
- Authentication middleware
- Error handling

**Wallet System** (`sdk/wallet.go`):
- Wallet creation and management
- Private key handling
- Transaction signing
- Balance calculation

**Helios Consensus** (`sdk/helios.go`):
- Three-stage consensus algorithm
- Proof generation and validation
- Sidechain routing
- Difficulty adjustment

## 🔧 Development Workflow

### 1. Feature Development

**Create Feature Branch**:
```bash
git checkout -b feature/your-feature-name
```

**Make Changes**:
- Follow coding standards
- Write tests for new functionality
- Update documentation
- Test thoroughly

**Commit Changes**:
```bash
git add .
git commit -m "feat: add new feature description"
```

**Push and Create PR**:
```bash
git push origin feature/your-feature-name
# Create pull request on GitHub
```

### 2. Bug Fixes

**Create Bug Fix Branch**:
```bash
git checkout -b fix/bug-description
```

**Fix the Issue**:
- Reproduce the bug
- Write test to demonstrate bug
- Implement fix
- Verify fix works

**Commit Fix**:
```bash
git commit -m "fix: resolve bug description"
```

### 3. Testing

**Run All Tests**:
```bash
make test
```

**Run Specific Tests**:
```bash
go test ./sdk -run TestSpecificFunction
```

**Run with Coverage**:
```bash
go test ./sdk -cover
```

**Run Performance Tests**:
```bash
go test ./sdk -bench=.
```

### 4. Code Review

**Before Submitting**:
- Run all tests
- Check code formatting
- Review documentation
- Test manually

**Pull Request Checklist**:
- [ ] Tests pass
- [ ] Code follows standards
- [ ] Documentation updated
- [ ] No breaking changes
- [ ] Performance considered

## 📝 Coding Standards

### Go Code Style

**Follow Go Conventions**:
- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use meaningful variable names
- Write clear comments

**Example**:
```go
// CreateBlock creates a new block with the given transactions
func (bc *Blockchain) CreateBlock(transactions []Transaction) (*Block, error) {
    if len(transactions) == 0 {
        return nil, errors.New("no transactions provided")
    }
    
    block := &Block{
        Index:        bc.GetLatestBlock().Index + 1,
        Timestamp:    time.Now().Unix(),
        Transactions: transactions,
        PreviousHash: bc.GetLatestBlock().Hash,
    }
    
    return block, nil
}
```

### Error Handling

**Use Proper Error Handling**:
```go
// Good: Return errors
func (w *Wallet) SignTransaction(tx Transaction) error {
    if w.privateKey == nil {
        return errors.New("wallet not unlocked")
    }
    
    signature, err := w.privateKey.Sign(tx.Hash())
    if err != nil {
        return fmt.Errorf("failed to sign transaction: %w", err)
    }
    
    tx.SetSignature(signature)
    return nil
}
```

### Testing Standards

**Write Comprehensive Tests**:
```go
func TestWallet_Create(t *testing.T) {
    // Arrange
    passphrase := "test-passphrase-123"
    
    // Act
    wallet, err := CreateWallet(passphrase)
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, wallet)
    assert.NotEmpty(t, wallet.GetAddress())
    assert.True(t, wallet.IsEncrypted())
}
```

**Test Categories**:
- **Unit Tests**: Test individual functions
- **Integration Tests**: Test component interactions
- **Performance Tests**: Benchmark critical functions
- **Security Tests**: Verify cryptographic operations

## 🔐 Security Guidelines

### Cryptographic Code

**Use Secure Random**:
```go
import "crypto/rand"

// Good: Use crypto/rand
privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

// Bad: Don't use math/rand for crypto
privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.New(rand.NewSource(1)))
```

**Validate Inputs**:
```go
func (w *Wallet) SignTransaction(tx Transaction) error {
    // Validate transaction
    if err := tx.Validate(); err != nil {
        return fmt.Errorf("invalid transaction: %w", err)
    }
    
    // Validate wallet state
    if !w.IsUnlocked() {
        return errors.New("wallet is locked")
    }
    
    return w.sign(tx)
}
```

### API Security

**Input Validation**:
```go
func (api *API) CreateTransaction(w http.ResponseWriter, r *http.Request) {
    var req CreateTransactionRequest
    
    // Validate JSON
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Validate fields
    if err := req.Validate(); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Process request
    // ...
}
```

## 🧪 Testing Strategy

### Test Organization

**Test File Structure**:
```
sdk/
├── blockchain.go
├── blockchain_test.go
├── wallet.go
├── wallet_test.go
└── ...
```

**Test Categories**:
- **Unit Tests**: Test individual functions
- **Integration Tests**: Test component interactions
- **Performance Tests**: Benchmark critical functions
- **Security Tests**: Verify cryptographic operations

### Test Examples

**Unit Test**:
```go
func TestBlockchain_AddBlock(t *testing.T) {
    bc := NewBlockchain()
    block := &Block{Index: 1, Hash: "test-hash"}
    
    err := bc.AddBlock(block)
    
    assert.NoError(t, err)
    assert.Len(t, bc.Blocks, 2) // Genesis + new block
}
```

**Integration Test**:
```go
func TestWallet_TransactionFlow(t *testing.T) {
    // Create wallets
    wallet1, _ := CreateWallet("pass1")
    wallet2, _ := CreateWallet("pass2")
    
    // Create blockchain
    bc := NewBlockchain()
    
    // Create transaction
    tx := &BankTransaction{
        From:   wallet1.GetAddress(),
        To:     wallet2.GetAddress(),
        Amount: 10.0,
    }
    
    // Sign and add transaction
    wallet1.SignTransaction(tx, "pass1")
    bc.AddTransaction(tx)
    
    // Mine block
    block := bc.MineBlock()
    assert.NotNil(t, block)
}
```

**Performance Test**:
```go
func BenchmarkWallet_SignTransaction(b *testing.B) {
    wallet, _ := CreateWallet("test-passphrase")
    tx := &BankTransaction{Amount: 10.0}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        wallet.SignTransaction(tx, "test-passphrase")
    }
}
```

## 📚 Documentation Standards

### Code Documentation

**Function Comments**:
```go
// CreateWallet creates a new encrypted wallet with the given passphrase.
// The wallet is encrypted using AES-GCM with a key derived from the passphrase
// using scrypt. Returns the wallet and any error encountered.
func CreateWallet(passphrase string) (*Wallet, error) {
    // Implementation...
}
```

**Package Comments**:
```go
// Package wallet provides secure wallet functionality for the blockchain.
// It includes wallet creation, encryption, transaction signing, and balance
// management. All wallets are encrypted using AES-GCM with scrypt key derivation.
package wallet
```

### API Documentation

**Endpoint Documentation**:
```go
// CreateTransaction godoc
// @Summary Create a new transaction
// @Description Create and broadcast a new transaction to the blockchain
// @Tags transactions
// @Accept json
// @Produce json
// @Param transaction body CreateTransactionRequest true "Transaction details"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/transaction/create [post]
func (api *API) CreateTransaction(w http.ResponseWriter, r *http.Request) {
    // Implementation...
}
```

## 🚀 Performance Guidelines

### Optimization Strategies

**Memory Management**:
```go
// Use object pools for frequently allocated objects
var blockPool = sync.Pool{
    New: func() interface{} {
        return &Block{}
    },
}

func getBlock() *Block {
    return blockPool.Get().(*Block)
}

func putBlock(block *Block) {
    block.Reset()
    blockPool.Put(block)
}
```

**Concurrent Processing**:
```go
func (bc *Blockchain) ProcessTransactions(transactions []Transaction) {
    const numWorkers = 4
    jobs := make(chan Transaction, len(transactions))
    results := make(chan error, len(transactions))
    
    // Start workers
    for i := 0; i < numWorkers; i++ {
        go func() {
            for tx := range jobs {
                results <- bc.processTransaction(tx)
            }
        }()
    }
    
    // Send jobs
    for _, tx := range transactions {
        jobs <- tx
    }
    close(jobs)
    
    // Collect results
    for range transactions {
        if err := <-results; err != nil {
            // Handle error
        }
    }
}
```

## 🔧 Debugging

### Debug Tools

**Use Delve Debugger**:
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug with delve
dlv debug cmd/chaind/main.go
```

**VS Code Debug Configuration**:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Blockchain",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/chaind/main.go",
      "args": []
    }
  ]
}
```

### Logging

**Structured Logging**:
```go
import "log"

func (bc *Blockchain) AddBlock(block *Block) error {
    log.Printf("Adding block %d with %d transactions", 
        block.Index, len(block.Transactions))
    
    // Implementation...
    
    log.Printf("Successfully added block %d", block.Index)
    return nil
}
```

## 📊 Code Quality

### Static Analysis

**Run Linters**:
```bash
# Install linters
go install golang.org/x/lint/golint@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run checks
golint ./...
staticcheck ./...
go vet ./...
```

**Pre-commit Hooks**:
```bash
#!/bin/sh
# .git/hooks/pre-commit

# Run tests
make test

# Run linters
golint ./...
staticcheck ./...
go vet ./...

# Check formatting
gofmt -d .
```

### Code Coverage

**Coverage Targets**:
- **Core Functions**: 90%+
- **API Endpoints**: 85%+
- **Overall Project**: 80%+

**Generate Coverage Report**:
```bash
go test ./sdk -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 🤝 Contributing Guidelines

### Pull Request Process

1. **Fork the Repository**
2. **Create Feature Branch**: `git checkout -b feature/amazing-feature`
3. **Make Changes**: Follow coding standards
4. **Write Tests**: Ensure good test coverage
5. **Update Documentation**: Keep docs current
6. **Commit Changes**: Use conventional commits
7. **Push to Branch**: `git push origin feature/amazing-feature`
8. **Create Pull Request**: Provide clear description

### Commit Message Format

**Conventional Commits**:
```
type(scope): description

feat(wallet): add multi-signature support
fix(api): resolve authentication bug
docs(readme): update installation instructions
test(blockchain): add performance benchmarks
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Build/tool changes

### Review Process

**Code Review Checklist**:
- [ ] Code follows standards
- [ ] Tests are comprehensive
- [ ] Documentation is updated
- [ ] Performance is considered
- [ ] Security is addressed
- [ ] No breaking changes

## 🔮 Future Development

### Planned Features

**Short Term**:
- CLI interface
- Advanced P2P networking
- Additional sidechain protocols
- Performance optimizations

**Medium Term**:
- Mobile wallet app
- Web wallet interface
- Smart contract support
- Cross-chain functionality

**Long Term**:
- Quantum-resistant cryptography
- Zero-knowledge proofs
- Advanced consensus algorithms
- Enterprise features

---

**For more information about specific components, see the [Architecture](architecture.md) and [API Reference](api.md) documentation.** 