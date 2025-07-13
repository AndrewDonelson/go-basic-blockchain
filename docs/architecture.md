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

### 5. Persistence

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

```
1. Node starts up
   ↓
2. Load local blockchain
   ↓
3. Connect to peers
   ↓
4. Request missing blocks
   ↓
5. Validate received blocks
   ↓
6. Update local chain
   ↓
7. Broadcast new blocks
   ↓
8. Maintain network state
```

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
- API key authentication
- Session management
- Rate limiting (planned)
- Input validation

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

```bash
# API Configuration
API_PORT=8200
API_HOST=localhost

# Blockchain Configuration
MINING_DIFFICULTY=4
BLOCK_REWARD=50
BLOCK_TIME=10

# Network Configuration
P2P_PORT=8100
MAX_PEERS=10

# Security Configuration
SCRYPT_N=16384  # Test mode
SCRYPT_N=1048576  # Production mode
```

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