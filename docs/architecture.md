# Architecture Overview

This document provides a comprehensive overview of the Go Basic Blockchain architecture, including system design, component relationships, and data flow.

## 🏗️ System Architecture

### High-Level Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Interface │    │   API Layer     │    │  Blockchain     │
│   (Port 8200)   │◄──►│   (REST/JSON)   │◄──►│   Core          │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                ▼                       ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   P2P Network   │    │   Persistence   │
                       │   (Peers)       │    │   (Local Files) │
                       └─────────────────┘    └─────────────────┘
```

### Core Components

1. **Blockchain Core**: Main blockchain implementation
2. **API Layer**: RESTful API with authentication
3. **Web Interface**: User-friendly web UI
4. **P2P Network**: Peer-to-peer communication
5. **Persistence**: Local data storage
6. **Wallet System**: Cryptographic key management
7. **Helios Consensus**: Advanced consensus algorithm

## 🔧 Component Details

### 1. Blockchain Core

**Location**: `sdk/blockchain.go`

**Responsibilities**:
- Block creation and validation
- Transaction processing
- Chain state management
- Mining operations
- Helios consensus integration

**Key Structures**:
```go
type Blockchain struct {
    Blocks          []*Block
    PendingTxs      []Transaction
    Helios          *HeliosConsensus
    SidechainRouter *SidechainRouter
    // ... other fields
}
```

**Data Flow**:
1. Transactions submitted via API
2. Transactions added to pending pool
3. Mining process creates new blocks
4. Blocks validated and added to chain
5. Chain state updated

### 2. API Layer

**Location**: `sdk/api.go`, `sdk/apiEndpointsAccount.go`

**Responsibilities**:
- RESTful endpoint handling
- Request/response serialization
- Authentication middleware
- Error handling and logging

**Key Endpoints**:
- `GET /api/blockchain/status` - Blockchain status
- `POST /api/transaction/create` - Create transactions
- `GET /api/wallet/balance/{address}` - Wallet balance
- `POST /api/mining/start` - Start mining
- `GET /api/blockchain/blocks` - Get blocks

**Authentication**:
- API key middleware for protected endpoints
- Session-based authentication for web interface
- Rate limiting (planned)

### 3. Web Interface

**Location**: `sdk/html5.go`

**Responsibilities**:
- User-friendly blockchain explorer
- Real-time blockchain status
- Transaction creation interface
- Mining controls
- Wallet management

**Features**:
- Block explorer with search
- Transaction history
- Real-time network status
- Interactive mining controls
- Wallet creation and management

### 4. P2P Network

**Location**: `sdk/p2p.go`, `sdk/node.go`

**Responsibilities**:
- Peer discovery and connection
- Block propagation
- Transaction broadcasting
- Network synchronization
- Consensus communication

**Protocol**:
- TCP-based peer communication
- JSON message format
- Automatic peer discovery
- Connection management

### 5. State (UTXO set)

The authoritative record of balances, derived deterministically from the blocks
and held in memory. See [utxo.md](utxo.md). It is never loaded from disk: it is
rebuilt by replaying the chain, so it cannot drift from the blocks.

### 6. Persistence

**Location**: `sdk/localstorage.go`

**Responsibilities**:
- Block storage
- Wallet file management
- Transaction history
- Configuration persistence
- State recovery

**Storage Format**:
- JSON files for human readability
- Encrypted wallet files
- Block chain files
- Configuration files

### 6. Wallet System

**Location**: `sdk/wallet.go`, `sdk/vault.go`

**Responsibilities**:
- Private key generation and storage
- Transaction signing
- Balance calculation
- Address generation
- Security management

**Security Features**:
- AES-GCM encryption
- Scrypt key derivation
- Secure random generation
- Password strength validation

### 7. Helios Consensus

**Location**: `sdk/helios.go`

**Responsibilities**:
- Three-stage consensus algorithm
- Proof generation and validation
- Sidechain routing
- Difficulty adjustment
- Block finalization

**Stages**:
1. **Proof Generation**: Cryptographic proofs for transactions
2. **Sidechain Routing**: Protocol-specific transaction routing
3. **Block Finalization**: Proof verification and block addition

## 📊 Data Flow

### Transaction Processing

```
1. User submits transaction via API
   ↓
2. API validates transaction format
   ↓
3. Transaction added to pending pool
   ↓
4. Mining process selects transactions
   ↓
5. Helios consensus processes transactions
   ↓
6. New block created with transactions
   ↓
7. Block validated and added to chain
   ↓
8. Chain state updated
   ↓
9. Response sent to user
```

### Block Creation

```
1. Mining process starts
   ↓
2. Pending transactions selected
   ↓
3. Helios Stage 1: Proof generation
   ↓
4. Helios Stage 2: Sidechain routing
   ↓
5. Block header created
   ↓
6. Proof-of-work mining
   ↓
7. Helios Stage 3: Block finalization
   ↓
8. Block added to chain
   ↓
9. Network propagation
```

### Network Synchronization

Implemented -- see [Chain Synchronisation](sync.md) for the protocol and its
limits.

```
1. Node starts up
   ↓
2. Load local blockchain            ← LoadExistingBlocks
   ↓
3. Connect to peers                 ← handshake, seed connection, GET_NODES
   ↓
4. Request missing blocks           ← Syncer: GET_STATUS then GET_BLOCKS
   ↓
5. Validate received blocks         ← AcceptBlock (structure, txs, proof of work)
   ↓
6. Update local chain               ← head-extension only; no reorg
   ↓
7. Broadcast new blocks             ← ANNOUNCE_BLOCK on mining
   ↓
8. Maintain network state           ← peer refresh on every handshake
```

**Still missing: fork choice.** A block that does not extend the current head is
refused rather than compared by cumulative work. There is no orphan pool and no
rollback, so two nodes that mine simultaneously diverge permanently. Sync closes
gaps; it does not resolve competing histories.

**Peer authentication and session encryption are implemented** -- see
[P2P Security](p2p-security.md). Node IDs are the hash of a public key, peers
prove possession of that key during a mutually authenticated handshake, and the
session is AES-256-GCM keyed by ECDH over the same identities.

## 🔐 Security Architecture

### Cryptographic Implementations

**Hashing**:
- SHA-256 for block hashing
- SHA-256 for transaction hashing
- SHA-256 for Merkle tree construction

**Encryption**:
- AES-GCM for wallet encryption
- Scrypt for key derivation
- ECDSA for transaction signing

**Key Management**:
- Secure random generation
- Encrypted storage
- Password-based protection
- Recovery mechanisms

### Authentication & Authorization

**API Security**:
- API key authentication that **fails closed** — with no key configured, every
  authenticated request is rejected. There is no built-in fallback credential.
- Constant-time key comparison (`crypto/subtle`), so keys are not recoverable by
  timing.
- Keys are stored as SHA-256 hashes, never in plaintext.
- Rate limiting per source address on every authenticated and credential endpoint.
- Request body size caps (1 MiB) and full server timeouts (read-header, read,
  write, idle) plus a header size cap.
- Input validation on every handler.
- **No TLS.** Terminate it in front of the node.

**Wallet Security**:
- Strong password requirements
- Encrypted private keys
- Secure key derivation
- Backup and recovery

## 🚀 Performance Considerations

### Optimization Strategies

**Memory Management**:
- Efficient data structures
- Garbage collection optimization
- Memory pooling for transactions

**Network Optimization**:
- Connection pooling
- Message batching
- Compression for large data

**Storage Optimization**:
- Efficient JSON serialization
- Indexed data structures
- Compressed storage format

### Scalability Features

**Horizontal Scaling**:
- Stateless API design
- Load balancer support
- Database abstraction layer

**Vertical Scaling**:
- Concurrent processing
- Memory optimization
- CPU utilization

## 🔧 Configuration

### Environment Variables

The real variable names are in [`.env.example`](../.env.example); the list below
previously used names (`API_PORT`, `MINING_DIFFICULTY`, `SCRYPT_N`, …) that the
code never read.

```bash
# API / network
API_HOSTNAME=:8200
P2P_HOSTNAME=:8201
ENABLE_API=true

# Blockchain
DIFFICULTY=4                 # 1..255
BLOCK_TIME=20                # seconds
MAX_BLOCK_SIZE=1000000
TRANSACTION_FEE=0.05
MINER_REWARD_PCT=50.00
DEV_REWARD_PCT=50.00

# Security -- REQUIRED, no defaults
BLOCKCHAIN_API_KEY=          # hex; without it the API rejects everything
BLOCKCHAIN_SERVER_SEED=      # hex
NODE_WALLET_PASSPHRASE=      # else one is generated and logged once
TRUST_PROXY_HEADERS=false    # X-Forwarded-For is client-controlled
```

scrypt cost is **not** an environment variable. It is chosen at wallet-creation
time and recorded in the wallet's `EncryptionParams`, so a wallet always decrypts
with the parameters it was encrypted with.

### Configuration Files

**Blockchain Config**:
```json
{
  "mining_difficulty": 4,
  "block_reward": 50,
  "block_time": 10,
  "max_transactions_per_block": 1000
}
```

**Network Config**:
```json
{
  "p2p_port": 8100,
  "max_peers": 10,
  "discovery_enabled": true,
  "sync_interval": 30
}
```

## 🔄 State Management

### Blockchain State

**Global State**:
- Current block height
- Total difficulty
- Network peers
- Mining status

**Local State**:
- Wallet balances
- Transaction history
- Block cache
- Network connections

### State Transitions

**Block Addition**:
1. Validate new block
2. Update chain state
3. Process transactions
4. Update balances
5. Broadcast to network

**Transaction Processing**:
1. Validate transaction
2. Check balances
3. Update pending pool
4. Broadcast to network
5. Update local state

## 🧪 Testing Architecture

### Test Categories

**Unit Tests**:
- Individual component testing
- Function-level validation
- Error condition testing

**Integration Tests**:
- Component interaction testing
- End-to-end workflows
- API endpoint testing

**Performance Tests**:
- Load testing
- Memory profiling
- Network simulation

### Test Infrastructure

**Test Data**:
- Mock blockchain data
- Test wallets
- Sample transactions
- Network simulation

**Test Utilities**:
- Test helpers
- Mock implementations
- Performance benchmarks
- Coverage reporting

## 📈 Monitoring & Observability

### Metrics Collection

**Performance Metrics**:
- Block creation rate
- Transaction processing time
- Memory usage
- Network latency

**Business Metrics**:
- Active wallets
- Transaction volume
- Network size
- Mining difficulty

### Logging

**Log Levels**:
- DEBUG: Detailed debugging information
- INFO: General operational information
- WARN: Warning conditions
- ERROR: Error conditions

**Log Categories**:
- Blockchain operations
- Network communication
- API requests
- Security events

---

**This architecture provides a solid foundation for educational blockchain development while maintaining production-ready code quality and extensibility.** 