# Introduction to Go Basic Blockchain

> For a full technical and community overview, see the [White Paper](WHITEPAPER.md).

## What is Go Basic Blockchain?

Go Basic Blockchain is an educational blockchain implementation written in Go that demonstrates core blockchain concepts from scratch. This project serves as a comprehensive learning tool for developers, students, and blockchain enthusiasts who want to understand the fundamental mechanics of blockchain systems.

## 🎯 Project Goals

### Educational Focus
- **Learn by Doing**: Every component is implemented from the ground up
- **No Black Boxes**: Clear, readable code with detailed explanations
- **Hands-on Experience**: Practical understanding of blockchain mechanics
- **Real-world Concepts**: Implements actual blockchain features and algorithms

### Advanced Features
- **Helios Consensus**: Sophisticated three-stage consensus algorithm
- **Sidechain Routing**: Transaction routing through specialized protocols
- **Proof Validation**: Cryptographic proof generation and verification
- **Dynamic Difficulty**: Adaptive mining difficulty adjustment

## 🏗️ Key Components

### Core Blockchain
- **Blocks**: Cryptographic containers for transaction data
- **Transactions**: Multiple types (BANK, MESSAGE, COINBASE, PERSIST)
- **Mining**: Proof-of-work consensus with adjustable difficulty
- **Persistence**: Local storage for blockchain state

### Advanced Consensus (Helios)
- **Stage 1**: Proof generation for transaction validation
- **Stage 2**: Sidechain routing through specialized protocols
- **Stage 3**: Block finalization with proof verification
- **Difficulty Adjustment**: Dynamic parameterized targets

### Wallet System
- **Encryption**: AES-GCM encryption with scrypt key derivation
- **Key Management**: Secure private key storage and recovery
- **Transaction Signing**: Cryptographic signature generation
- **Balance Tracking**: Real-time balance calculation

### Network Layer
- **P2P Communication**: Peer-to-peer networking framework
- **API Layer**: RESTful endpoints with authentication
- **Node Discovery**: Automatic peer discovery and connection
- **Block Propagation**: Efficient block sharing across network

## 🚀 Why This Project?

### For Learners
- **Clear Code**: Well-documented, readable implementations
- **Step-by-Step**: Progressive complexity from basic to advanced
- **Educational Comments**: Detailed explanations of design decisions
- **Test Coverage**: Comprehensive testing for understanding

### For Developers
- **Production-Ready Code**: Professional implementation patterns
- **Modular Design**: Clean separation of concerns
- **Extensible Architecture**: Easy to add new features
- **Performance Optimized**: Fast test suite and efficient algorithms

### For Researchers
- **Advanced Consensus**: Helios algorithm implementation
- **Sidechain Protocols**: Specialized transaction routing
- **Proof Systems**: Cryptographic proof validation
- **Scalability Features**: Designed for future expansion

## 📊 Current Status

### Implementation Progress

| Component | Status |
|---|---|
| Core blockchain (blocks, mining, persistence) | ✅ Working |
| Helios consensus | ✅ Working — deterministic proof, verified on every block |
| Wallet system | ✅ Working — encrypted at rest, atomic writes, recorded KDF parameters |
| UTXO state model | ✅ Working — authoritative balances, double-spend prevention |
| API layer | ✅ Working — fails closed, rate limited, paginated |
| Chain synchronisation | ✅ Working — pull from longer peers, push mined blocks |
| Peer authentication | ✅ Working — mutual auth, forward-secret sessions |
| Fork choice / reorganisation | ✅ Working — heaviest-chain by cumulative work |

### Performance Metrics
- **Test Coverage**: 71.9% (`sdk`), 85–97% across the Helios packages
- **Test Performance**: ~17 seconds; ~85 seconds under the race detector
- **Memory Usage**: Optimized for educational use

### What is deliberately not built yet

This matters if you are reading the code to learn from it — these are absent, not
broken:

- **Transaction finality.** Any block within 100 of the head can still be
  reorganised away. Wait for confirmations.
- **Explicit transaction inputs.** The UTXO set selects inputs deterministically
  at apply time, so a transaction cannot be validated in isolation — you need the
  set as of its block.
- **Wiping ephemeral keys from memory.** They are dropped once the session key is
  derived, but Go cannot zero the underlying memory.

## 🎓 Learning Path

### Beginner Level
1. **Understanding Blocks**: Learn how blocks store and link data
2. **Basic Transactions**: Create and validate simple transactions
3. **Wallet Operations**: Create, encrypt, and use wallets
4. **Mining Basics**: Understand proof-of-work consensus

### Intermediate Level
1. **Network Communication**: Explore P2P networking
2. **API Integration**: Use RESTful endpoints
3. **Advanced Transactions**: Work with different transaction types
4. **Security Features**: Understand cryptographic implementations

### Advanced Level
1. **Helios Consensus**: Study the three-stage algorithm
2. **Sidechain Routing**: Implement specialized protocols
3. **Proof Validation**: Understand cryptographic proofs
4. **Performance Optimization**: Scale and optimize the system

## 🔧 Technology Stack

### Core Technologies
- **Go**: Primary programming language
- **JSON**: Data serialization and storage
- **AES-GCM**: Wallet encryption
- **Scrypt**: Key derivation function
- **SHA-256**: Cryptographic hashing

### Advanced Features
- **Helios Algorithm**: Custom consensus implementation
- **Sidechain Router**: Protocol-specific transaction routing
- **Proof Generation**: Cryptographic proof systems
- **Dynamic Difficulty**: Adaptive mining parameters

## 📚 Documentation Structure

This project includes comprehensive documentation:

- **[Quick Start](quickstart.md)**: Get up and running quickly
- **[Architecture](architecture.md)**: System design and components
- **[API Reference](api.md)**: Complete API documentation
- **[Wallet Guide](wallet.md)**: Wallet creation and management
- **[Helios Consensus](helios.md)**: Advanced consensus algorithm
- **[Development Guide](development.md)**: Contributing guidelines
- **[White Paper](WHITEPAPER.md)**: Project vision, architecture, and standards

## 🤝 Contributing

We welcome contributions from the community! Whether you're:
- **Fixing bugs**: Report and fix issues
- **Adding features**: Implement new functionality
- **Improving docs**: Enhance documentation
- **Optimizing performance**: Make the system faster

See our [Development Guide](development.md) for details on contributing.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.

---

**Ready to start your blockchain journey? Follow the [Quick Start](quickstart.md) guide to get up and running!** 