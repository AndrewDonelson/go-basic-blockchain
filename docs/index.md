# Go Basic Blockchain - Documentation

Welcome to the comprehensive documentation for the Go Basic Blockchain project. This educational blockchain implementation demonstrates core blockchain concepts from scratch, including the advanced Helios consensus algorithm.

## 📚 Documentation Structure

### Getting Started
- **[Introduction](intro.md)** - Getting started guide and project overview
- **[Quick Start](quickstart.md)** - Fast setup and first steps
- **[White Paper](WHITEPAPER.md)** - Project vision, architecture, and community standards

### Core Concepts
- **[Architecture](architecture.md)** - System design and component overview
- **[Helios Consensus](helios.md)** - The proof-of-work algorithm, and why determinism is what makes it work
- **[Chain Synchronisation](sync.md)** - How nodes exchange blocks
- **[Fork Choice & Reorganisation](forkchoice.md)** - How competing histories are resolved
- **[Dynamic Difficulty](difficulty.md)** - How the chain retargets itself
- **[UTXO Set](utxo.md)** - The authoritative record of who owns what
- **[Mempool Policy](mempool.md)** - Fee ordering, eviction, and block size limits
- **[Supply & Incentives](supply.md)** - Fixed supply, genesis-only minting, fee rewards
- **[Wallet Recovery](recovery.md)** - BIP-39 phrases and deterministic key derivation
- **[Observability](metrics.md)** - The /metrics endpoint, counters and hash rate
- **[P2P Security](p2p-security.md)** - Peer authentication, forward secrecy, session encryption

### User Guides
- **[API Reference](api.md)** - The REST API as implemented
- **[Wallet Guide](wallet.md)** - Wallet creation, management, and security

### Development
- **[Development Guide](development.md)** - Contributing, coding standards, and security guidelines drawn from the audit
- **[Testing Guide](testing.md)** - Test suite and coverage information

### Learning
- **[Learning Course](../learn/README.md)** - 19 sections, beginner to advanced
- **[Security Case Studies](../learn/SECURITY_CASE_STUDIES.md)** - Thirteen real defects from this codebase, with causes and fixes

### Not yet written

These were listed here as links, but the files were never created. They are
recorded as gaps rather than as 404s:

`fundamentals.md`, `mining.md`, `network.md`, `deployment.md`,
`troubleshooting.md`, `sidechains.md`, `security.md`, `performance.md`,
`extending.md`

## 🚀 Quick Navigation

### For New Users
1. Start with [Introduction](intro.md) to understand the project
2. Follow the [Quick Start](quickstart.md) guide to get running
3. Explore [Wallet Guide](wallet.md) for basic operations
4. Check [API Reference](api.md) for programmatic access
5. Read the [White Paper](WHITEPAPER.md) for project vision

### For Developers
1. Review [Architecture](architecture.md) for system understanding
2. Study [Helios Consensus](helios.md) for advanced features
3. Follow [Development Guide](development.md) for contributing
4. Use [Testing Guide](testing.md) for quality assurance
5. Reference the [White Paper](WHITEPAPER.md) for standards

### For Advanced Users
4. Consult the [White Paper](WHITEPAPER.md) for governance and roadmap

## 📊 Project Status

A **single-node educational blockchain**: it mines, validates and persists a chain
correctly, but does not yet form a network.

| Area | Status |
|---|---|
| Block production & persistence | ✅ Working |
| Proof of work (Helios) | ✅ Working — deterministic and verified on every block |
| Transaction signing | ✅ Working — covers all protocol fields |
| REST API | ✅ Working — authenticated, rate limited, paginated |
| P2P peer discovery | ✅ Working |
| Chain synchronisation | ✅ Working — see [sync.md](sync.md) |
| Peer authentication | ✅ Working — mutual auth + forward-secret sessions ([p2p-security.md](p2p-security.md)) |
| Fork choice / reorg | ✅ Working — heaviest-chain, see [forkchoice.md](forkchoice.md) |
| Dynamic difficulty | ✅ Working — retargets from chain history, validated per branch ([difficulty.md](difficulty.md)) |
| Mempool policy | ✅ Working — fee-rate ordering, bounded, size-capped blocks ([mempool.md](mempool.md)) |
| Supply & miner rewards | ✅ Working — fixed supply, genesis-only minting, fees split to miner/dev ([supply.md](supply.md)) |
| Wallet recovery | ✅ Working — every wallet derives from a BIP-39 phrase ([recovery.md](recovery.md)) |
| Observability | ✅ Working — Prometheus /metrics, real hash rate ([metrics.md](metrics.md)) |
| UTXO state model | ✅ Working — authoritative balances, double-spend prevention ([utxo.md](utxo.md)) |

- **Test Coverage**: 69.7% (`sdk`), 85–97% across the Helios packages
- **Test Performance**: ~17 seconds; ~85 seconds under `-race`
- **Documentation**: ✅ Comprehensive

## 🔗 External Resources

- **[GitHub Repository](https://github.com/yourusername/go-basic-blockchain)**
- **[Issue Tracker](https://github.com/yourusername/go-basic-blockchain/issues)**
- **[Contributing Guidelines](../CONTRIBUTING.md)**
- **[License](../LICENSE)**

## 📝 Documentation Updates

This documentation is actively maintained and updated with each major release. For the latest information, always refer to the documentation in the repository.

---

**Ready to explore? Start with the [Introduction](intro.md) and dive into the world of blockchain technology!** 