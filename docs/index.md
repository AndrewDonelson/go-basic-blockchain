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
- **[Verifiable Delay Function](vdf.md)** - Wesolowski VDF over a class group
- **[Memory Hardness](memory-hardness.md)** - Argon2id for stage 1
- **[Sidechains](sidechains.md)** - Per-game block space, publisher identity, and anchoring
- **[UTXO Set](utxo.md)** - The authoritative record of who owns what
- **[Mempool Policy](mempool.md)** - Fee ordering, replace-by-fee, eviction, block size
- **[Nonces & Replay Protection](nonces.md)** - Per-sender sequencing
- **[Supply & Incentives](supply.md)** - Fixed supply, the reserve-funded block subsidy, fee rewards
- **[Wallet Recovery](recovery.md)** - BIP-39 phrases and deterministic key derivation
- **[Observability](metrics.md)** - The /metrics endpoint, counters and hash rate
- **[Performance](performance.md)** - Hot-path indexes and their benchmarks
- **[P2P Security](p2p-security.md)** - Peer authentication, forward secrecy, session encryption
- **[API TLS](api-tls.md)** - Serving the REST API over HTTPS

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
`troubleshooting.md`, `security.md`, `extending.md`

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
| Verifiable delay (stage 2) | ✅ Working — Wesolowski VDF, class group, no trusted setup ([vdf.md](vdf.md)) |
| Proof of work (Helios) | ✅ Working — deterministic and verified on every block |
| Transaction signing | ✅ Working — covers all protocol fields |
| REST API | ✅ Working — authenticated, rate limited, paginated |
| P2P peer discovery | ✅ Working |
| Chain synchronisation | ✅ Working — see [sync.md](sync.md) |
| Peer authentication | ✅ Working — mutual auth + forward-secret sessions ([p2p-security.md](p2p-security.md)) |
| Fork choice / reorg | ✅ Working — heaviest-chain, see [forkchoice.md](forkchoice.md) |
| Dynamic difficulty | ✅ Working — retargets from chain history, validated per branch ([difficulty.md](difficulty.md)) |
| Mempool policy | ✅ Working — fee-rate ordering, replace-by-fee, bounded, size-capped ([mempool.md](mempool.md)) |
| Replay protection | ✅ Working — per-sender nonce sequencing ([nonces.md](nonces.md)) |
| Supply & miner rewards | ✅ Working — fixed supply, reserve-funded block subsidy, fee split ([supply.md](supply.md)) |
| Wallet recovery | ✅ Working — every wallet derives from a BIP-39 phrase ([recovery.md](recovery.md)) |
| Observability | ✅ Working — Prometheus /metrics, real hash rate ([metrics.md](metrics.md)) |
| REST API TLS | ✅ Working — optional HTTPS, TLS 1.2 floor, forward-secret suites ([api-tls.md](api-tls.md)) |
| Hot-path performance | ✅ Indexed — network lookups no longer scan the chain ([performance.md](performance.md)) |
| UTXO state model | ✅ Working — authoritative balances, double-spend prevention ([utxo.md](utxo.md)) |

- **Test Coverage**: 78.2% (`sdk`), 85–97% across the Helios packages
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