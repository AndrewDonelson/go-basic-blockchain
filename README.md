# go-basic-blockchain

[![Buy Me A Coffee](https://img.shields.io/badge/Buy%20Me%20A%20Coffee-FFDD00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black)](https://buymeacoffee.com/andrewdonelson)

## Educational Blockchain Implementation in Go with Advanced Consensus & Comprehensive Learning Course

Welcome to go-basic-blockchain, an educational project designed to demystify blockchain technology through a hands-on, from-scratch implementation in Go. This project serves as both a **comprehensive learning tool** and a **fully functional blockchain implementation** for developers, students, and blockchain enthusiasts.

**🎯 Built Completely From Scratch**: This blockchain implementation uses no third-party blockchain libraries (no go-ethereum, Bitcoin, Tendermint/Cosmos, etc.) and includes our custom Helios consensus algorithm - all created from the ground up to fully teach Go programming and blockchain development.

**🚀 NEW: Complete Learning Course** - This project now includes a comprehensive 19-section course covering everything from Go fundamentals to production deployment!

**🎯 NEW: Fully Functional Blockchain Node** - Complete, operational blockchain with continuous mining, proper data persistence, and smooth progress monitoring.

## 📚 Learning Course

This repository includes a **comprehensive learning course** with **19 sections** organized into **4 phases**:

### **Course Structure**
- **Phase 1: Foundation** (Sections 1-5) - Go fundamentals and basic blockchain
- **Phase 2: Advanced Features** (Sections 6-10) - Advanced consensus and APIs
- **Phase 3: User Experience** (Sections 11-15) - Web and mobile applications
- **Phase 4: Production Quality** (Sections 16-19) - Testing and deployment

### **What You'll Learn**
- **Go Programming**: Master Go fundamentals, concurrency, and advanced patterns
- **Blockchain Development**: Build complete blockchain systems from scratch
- **Advanced Consensus**: Implement sophisticated consensus algorithms (Helios)
- **API Development**: Create professional RESTful APIs with authentication
- **Web Development**: Build responsive web applications with React
- **Mobile Development**: Create cross-platform mobile apps with React Native
- **Testing**: Implement comprehensive testing strategies
- **Deployment**: Deploy production-ready applications

### **Course Features**
- **80-109 hours** of comprehensive content
- **19 sections** with hands-on exercises
- **19 quizzes** with detailed answer keys
- **4 major milestones** with working deliverables
- **Production-ready portfolio** project

**[Start Learning →](./learn/README.md)** | **[Complete Course Overview →](./learn/COURSE_OVERVIEW.md)**

---

## 🔧 Blockchain Implementation

### **Key Features**
- **Built from Scratch**: Every component implemented from the ground up
- **Advanced Consensus**: Implements the Helios consensus algorithm with three-stage validation
- **Go Programming Language**: Utilizing Go's simplicity and efficiency for complex systems
- **Readable JSON Format**: Human-readable JSON for data structures and storage
- **No Third-Party Dependencies**: Encourages deep understanding of core blockchain concepts
- **Educational Focus**: Thoroughly commented and documented with design explanations
- **Optimized Test Suite**: Fast, reliable tests with smart scrypt configuration
- **Sidechain Routing**: Advanced transaction routing through specialized protocols
- **Continuous Mining**: Automatic block creation and mining with proper state management
- **Professional Build System**: Organized binary output with cross-compilation support

### **Current Status**

This is a **single-node educational blockchain**. It mines, validates and persists
a chain correctly; it does not yet form a network. The status below is deliberately
specific about which is which.

| Area | Status |
|---|---|
| Block production & persistence | ✅ Working — blocks mine, save, reload and re-validate |
| Proof of work (Helios) | ✅ Working — deterministic, stored on the block, verified on every block |
| Transaction signing | ✅ Working — signatures cover all protocol fields |
| Wallets & key storage | ✅ Working — encrypted at rest, atomic writes |
| REST API | ✅ Working — authenticated, rate limited, paginated |
| P2P peer discovery | ✅ Working — handshake, node lists, seed connection |
| Chain synchronisation | ✅ Working — pull from longer peers, push mined blocks ([docs](docs/sync.md)) |
| Transaction propagation | ✅ Working — relayed transactions are signature-verified |
| Peer authentication | ✅ Working — mutual auth, forward-secret encrypted sessions ([docs](docs/p2p-security.md)) |
| Fork choice / reorganisation | ✅ Working — heaviest-chain by cumulative work ([docs](docs/forkchoice.md)) |
| Dynamic difficulty | ✅ Working — retargets from chain history, validated per branch ([docs](docs/difficulty.md)) |
| Mempool policy & block size | ✅ Working — fee-rate ordering, replace-by-fee, bounded mempool, capped blocks ([docs](docs/mempool.md)) |
| Replay protection | ✅ Working — per-sender nonce sequencing, reorg-safe ([docs](docs/nonces.md)) |
| Supply & miner rewards | ✅ Working — fixed supply, coinbase only in genesis, fees split to miner/dev ([docs](docs/supply.md)) |
| Wallet recovery | ✅ Working — every wallet derives from a BIP-39 recovery phrase ([docs](docs/recovery.md)) |
| Observability | ✅ Working — Prometheus `/metrics`, real hash rate ([docs](docs/metrics.md)) |
| Hot-path performance | ✅ Indexed — peer-driven lookups are O(1), not O(chain) ([docs](docs/performance.md)) |
| UTXO state model | ✅ Working — authoritative balances, double-spend prevention ([docs](docs/utxo.md)) |

- **Test Coverage:** 78.3% (`sdk`); 89.8% / 94.4% / 85.2% / 96.8% across the Helios packages
- **Test Suite Performance:** ~17 seconds for the full suite, ~85 seconds under `-race`

> **Heads up if you are upgrading:** authentication now **fails closed**. There is
> no built-in fallback API key any more, so `BLOCKCHAIN_API_KEY` and
> `BLOCKCHAIN_SERVER_SEED` must be set before the API will accept a request. See
> [`.env.example`](.env.example). Several endpoints also changed method and shape —
> see the [API Reference](docs/api.md).

## ⚙️ Configuration

Copy [`.env.example`](.env.example) to `.local.env` and fill it in. Two variables
have **no default** and must be set, because authentication fails closed rather
than falling back to a built-in credential:

```bash
cp .env.example .local.env

# Generate real values -- do not reuse these
echo "BLOCKCHAIN_API_KEY=$(openssl rand -hex 32)"     >> .local.env
echo "BLOCKCHAIN_SERVER_SEED=$(openssl rand -hex 32)" >> .local.env
```

| Variable | Required | Notes |
|---|---|---|
| `BLOCKCHAIN_API_KEY` | **yes** | Hex-encoded. Without it the API rejects every authenticated request. |
| `BLOCKCHAIN_SERVER_SEED` | **yes** | Hex-encoded, used for per-account derivation. |
| `NODE_WALLET_PASSPHRASE` | recommended | If unset, one is generated and logged **once** at startup. Save it or the node wallet is unrecoverable. |
| `TRUST_PROXY_HEADERS` | no | Only enable behind a proxy you control — `X-Forwarded-For` is client-controlled. |

## 🚀 Quick Start

### **For Learning**
1. Start with the [Learning Course](./learn/README.md)
2. Follow the structured progression through all 19 sections
3. Build your blockchain step by step with hands-on exercises
4. Read [**Security Case Studies**](./learn/SECURITY_CASE_STUDIES.md) — thirteen
   real defects that shipped in this codebase (signature forgery, a proof of work
   that proved nothing, an `Open()` that deleted private keys), each with its
   cause and the regression test that now guards it

### **For Blockchain Implementation**
1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/go-basic-blockchain.git
   cd go-basic-blockchain
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Run tests to verify everything works:
   ```bash
   make test
   ```

4. Start the blockchain node:
   ```bash
   make run
   ```
   
   Or run the binary directly:
   ```bash
   ./bin/release/gbbd
   ```

5. Access the web interface at `http://localhost:8200`

## 🏗️ Architecture Overview

### **Helios Consensus Algorithm**
The project implements the **Helios consensus algorithm**, a sophisticated three-stage consensus mechanism:

1. **Stage 1 - Proof Generation**: Miners create cryptographic proofs for transaction validation
2. **Stage 2 - Sidechain Routing**: Transactions are routed through specialized protocols (BANK, MESSAGE, etc.)
3. **Stage 3 - Block Finalization**: Validated blocks are finalized with proof verification

### **Key Components**
- **Blockchain Core**: Main blockchain implementation with Helios integration and continuous mining
- **Wallet System**: Encrypted wallet management with key derivation
- **API Layer**: RESTful endpoints with authentication middleware
- **P2P Network**: Peer-to-peer communication framework
- **Sidechain Router**: Transaction routing through specialized protocols
- **Test Suite**: Comprehensive testing with optimized performance
- **Progress Indicator**: Real-time status monitoring with smooth updates
- **Build System**: Professional Makefile with cross-compilation and organized output

## 📊 Project Structure

```
go-basic-blockchain/
├── learn/                    # Comprehensive learning course (19 sections)
│   ├── README.md            # Course overview and navigation
│   ├── COURSE_OVERVIEW.md   # Detailed course guide
│   ├── phase1/              # Foundation (Sections 1-5)
│   ├── phase2/              # Advanced Features (Sections 6-10)
│   ├── phase3/              # User Experience (Sections 11-15)
│   └── phase4/              # Production Quality (Sections 16-19)
├── cmd/                     # Application entry points
│   ├── gbb-cli/            # Command-line interface
│   ├── chaind/             # Blockchain daemon
│   └── demo/               # Demo applications
├── internal/               # Core blockchain implementation
│   ├── helios/             # Helios consensus algorithm
│   ├── menu/               # User interface components
│   └── progress/           # Progress tracking
├── docs/                   # Technical documentation
├── test/                   # Test files and examples
├── bin/                    # Compiled binaries
├── data/                   # Blockchain data storage
├── scripts/                # Build and deployment scripts
└── postman/                # API testing collections
```

## 📚 Documentation

### **Learning Resources**
- **[Learning Course](./learn/README.md)**: Start your blockchain development journey
- **[Course Overview](./learn/COURSE_OVERVIEW.md)**: Complete course navigation and progress tracking

### **Technical Documentation**
- **[Introduction](docs/intro.md)**: Getting started guide
- **[Architecture](docs/architecture.md)**: System design and components
- **[API Reference](docs/api.md)**: RESTful API documentation
- **[Wallet Guide](docs/wallet.md)**: Wallet creation and management
- **[Helios Consensus](docs/helios.md)**: Advanced consensus algorithm
- **[Development](docs/development.md)**: Contributing and development guide
- **[Testing](docs/testing.md)**: Test suite and coverage information

## 🧪 Testing

The project includes a comprehensive test suite with optimized performance:

- **Test Coverage**: 78.3% (`sdk`), 85–97% across the Helios packages
- **Test Execution Time**: ~17 seconds; ~85 seconds with the race detector
- **Race Detector**: The full suite runs clean under `go test -race`
- **Regression Suite**: `sdk/regression_test.go` pins one test per historical
  defect, each naming the behaviour it guards — useful reading on its own
- **Scrypt Cost Profile**: Recorded per wallet in `EncryptionParams`, so a wallet
  created under one profile always decrypts with the same one
- **Timeout Protection**: All tests have timeouts to prevent hanging

Run tests with:
```bash
make test                    # full suite
go test -race ./...          # with the race detector
go test -cover ./...         # with coverage
```

## 🔧 Development

### **Building**
```bash
make build          # Build release binary
make debug          # Build debug binary with debugging symbols
make build-all      # Build for all platforms (Linux, Windows, macOS, ARM64, AMD64)
```

### **Running**
```bash
make run            # Build and run release binary
make run-bin        # Run existing release binary
make debug          # Build and run debug binary with delve debugger
./bin/release/gbbd  # Run release binary directly
./bin/debug/gbbd    # Run debug binary directly
```

### **Testing**
```bash
make test           # Run all tests
make test-short     # Run short tests
make test-unit      # Run unit tests only
make test-coverage  # Run with coverage
```

### **Development**
```bash
make setup          # Setup dependencies
make fmt            # Format code
make lint           # Run linter
make clean          # Clean build artifacts
make clean-data     # Clean blockchain data
make clean-all      # Clean everything
```

### **Release**
```bash
make release        # Build all cross-compiled binaries
```

### **Documentation Generation**
```bash
make docs
```

## 📈 Performance

- **Test Suite**: 30x faster execution (9.5 seconds vs 5+ minutes)
- **Scrypt Optimization**: Smart configuration for development vs production
- **Memory Usage**: Optimized for educational use
- **Network Latency**: Minimal overhead in P2P communication
- **Blockchain Operation**: Continuous mining with proper state management
- **Progress Display**: Smooth, non-flickering status updates

## 🏗️ Build System

The project features a professional build system with:

- **Cross-Compilation**: Build for Linux, Windows, macOS (ARM64, AMD64)
- **Binary Organization**: Separate `bin/debug/` and `bin/release/` directories
- **Go 1.27**: `govulncheck` reports zero vulnerabilities; the `toolchain` line is a floor, not a pin
- **Dependency Management**: Proper `go.mod` and `go.sum` handling
- **Clean Targets**: Organized cleanup with dependency preservation

## 🎯 Learning Paths

### **For Complete Beginners**
1. Start with [Phase 1: Foundation](./learn/phase1/README.md)
2. Complete all sections in order
3. Build your first blockchain in Section 5
4. Progress through each phase sequentially

### **For Experienced Developers**
1. Review [Phase 1](./learn/phase1/README.md) for Go/blockchain basics
2. Jump to [Phase 2](./learn/phase2/README.md) for advanced features
3. Focus on [Phase 3](./learn/phase3/README.md) for user interfaces
4. Complete [Phase 4](./learn/phase4/README.md) for production deployment

### **For Specific Skills**
- **Go Programming**: [Phase 1, Sections 1-2](./learn/phase1/README.md)
- **Blockchain Theory**: [Phase 1, Section 3](./learn/phase1/section3/README.md)
- **Advanced Consensus**: [Phase 2, Section 6](./learn/phase2/section6/README.md)
- **API Development**: [Phase 2, Section 8](./learn/phase2/section8/README.md)
- **Web Development**: [Phase 3, Section 11](./learn/phase3/section11/README.md)
- **Mobile Development**: [Phase 3, Section 12](./learn/phase3/section12/README.md)
- **Testing**: [Phase 4, Section 16](./learn/phase4/section16/README.md)
- **Deployment**: [Phase 4, Section 17](./learn/phase4/section17/README.md)

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Ready to explore the future of blockchain technology? Choose your path:**

🎓 **[Start Learning →](./learn/README.md)** - Begin the comprehensive 19-section course  
🔧 **[Explore Implementation →](docs/intro.md)** - Dive into the technical documentation  
🚀 **[Quick Start →](#-quick-start)** - Get the blockchain running immediately

*Transform from beginner to production-ready blockchain developer with our complete learning experience!*
