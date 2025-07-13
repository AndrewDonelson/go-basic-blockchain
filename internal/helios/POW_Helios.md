# Helios Proof of Work Algorithm Specification

## Overview

**Helios** is a novel hybrid CPU-based blockchain mining algorithm designed for energy efficiency and scalability. It combines memory-hard functions, time-locked puzzles, and cryptographic operations in a three-stage process, with protocol-based sidechains for specialized transaction processing.

## Architecture

### Two-Tier Blockchain Structure

- **Main Chain**: 20-second blocks using full Helios hybrid PoW
- **Side Chains**: Protocol-specific chains with 1-second blocks and lightweight PoW
- **Rollup Process**: Every 20 seconds, completed sidechain transactions are merkleized and committed to main chain

### Protocol-Based Routing

- Each transaction includes a `protocol_id` field (e.g., "bank", "msg", "contract", "iot")
- Transactions are routed to dedicated protocol sidechains
- Protocols are voted on before mainnet addition (development team has initial control)

## Helios Hybrid PoW Algorithm

### Stage 1: Memory Phase (40% of computational work)
- **Algorithm**: Argon2-inspired memory-hard function
- **Memory Requirements**: Dynamic scaling based on difficulty (starts at 64MB, scales up)
- **Optimization**: NUMA-aware memory access patterns
- **Purpose**: Favors general-purpose CPUs, resistant to ASIC development

### Stage 2: Time-Lock Phase (30% of computational work)
- **Algorithm**: Verifiable Delay Function (VDF) inspired sequential computation
- **Characteristics**: Cannot be parallelized, uses single-threaded CPU performance
- **Duration**: Scales with difficulty (base: 50ms, increases with network hash rate)
- **Purpose**: Energy efficient, prevents wasteful parallel computation

### Stage 3: Cryptographic Puzzle (30% of computational work)
- **Algorithm**: AES-NI optimized operations combined with hash-based proofs
- **Input**: Incorporates sidechain merkle roots from previous 20-second period
- **Target**: Dynamic difficulty adjustment based on combined network conditions
- **Purpose**: Leverages modern CPU cryptographic instructions

## Sidechain Specifications

### Protocol Examples

| Protocol ID | Block Time | PoW Type | Use Case |
|-------------|------------|----------|----------|
| `bank` | 2.0s | Medium security | Financial transactions |
| `msg` | 0.5s | Lightweight | Messaging/communication |
| `contract` | 1.0s | Gas-metered | Smart contracts |
| `iot` | 0.2s | Minimal | IoT/sensor data |

### Sidechain PoW
- **Algorithm**: Simplified single-stage version of Helios (cryptographic puzzle only)
- **Difficulty**: Independent per protocol, adjusted based on protocol activity
- **Validation**: Lightweight for fast block times

## Difficulty Adjustment

### Multi-Factor Adjustment System
- **Block Time Targeting**: Maintains 20-second main chain, 1-second average sidechain
- **Energy Consumption**: Estimates based on algorithm complexity metrics
- **Network Hash Rate**: Smoothed adjustments to prevent oscillations
- **Sidechain Load**: Adjusts main chain difficulty based on sidechain activity

### Adaptive Parameters
- Memory requirements scale with main chain difficulty
- Time-lock duration adjusts based on network conditions
- Sidechain difficulty independent per protocol

## Rollup Process

### Every 20 Seconds
1. **Sidechain Finalization**: All active sidechains finalize their current blocks
2. **Transaction Validation**: Verify all sidechain transactions are valid
3. **Merkle Tree Construction**: Create merkle tree of all completed transactions
4. **Main Chain Inclusion**: Merkle root included in main chain block header
5. **State Commitment**: Final state changes committed to main blockchain

### Conflict Resolution
- **Sidechain Forks**: Longest valid chain rule per protocol
- **Failed Rollups**: Incomplete sidechain states excluded from main chain
- **Cross-Protocol Dependencies**: Handled through atomic transaction batching

## Energy Efficiency Features

### Design Optimizations
- **Memory-bound operations** reduce power consumption vs. compute-bound
- **Time-locked puzzles** prevent wasteful parallel computation
- **Adaptive difficulty** considers energy consumption metrics
- **Early termination** for invalid solutions saves energy

### Performance Characteristics
- **CPU Optimized**: Leverages AES-NI, AVX instructions
- **Cache Friendly**: Data structures optimized for CPU cache hierarchy
- **NUMA Aware**: Memory access patterns optimized for multi-socket systems

## Project Structure

```
./internal/helios/
├── algorithm/          # Core Helios mining algorithm implementation
├── difficulty/         # Multi-factor difficulty adjustment logic
├── memory/            # Memory-hard function components (Argon2-inspired)
├── crypto/            # Cryptographic primitives and AES-NI optimizations
├── validation/        # Proof validation for main chain and sidechains
├── metrics/           # Performance and energy consumption tracking
├── sidechain/         # Protocol-specific sidechain management
│   ├── router/        # Protocol ID routing and transaction dispatch
│   ├── protocols/     # Protocol-specific configurations and rules
│   └── manager/       # Sidechain lifecycle and resource management
├── rollup/            # Sidechain-to-mainchain rollup logic and merkleization
└── timelock/          # Time-locked puzzle components and VDF implementation
```

## Implementation Phases

### Phase 1: Core Algorithm
- Implement three-stage hybrid PoW
- Basic difficulty adjustment
- Single-threaded mining prototype

### Phase 2: Sidechain Integration
- Protocol routing system
- Lightweight sidechain PoW
- Basic rollup mechanism

### Phase 3: Optimization & Governance
- Multi-threaded mining
- Advanced difficulty adjustment
- Protocol governance system
- Performance optimizations

## Security Considerations

### Attack Resistance
- **51% Attack**: Requires control of both main chain and relevant sidechains
- **Sidechain Attacks**: Isolated per protocol, limited main chain impact
- **Time-Lock Bypass**: Sequential nature prevents acceleration attacks
- **Memory Attacks**: NUMA optimization makes memory attacks expensive

### Validation
- **Proof Verification**: All three stages must be validated
- **Sidechain Integrity**: Merkle proofs ensure sidechain transaction validity
- **Cross-Chain Consistency**: Rollup process ensures consistent state

---

**Target Specifications:**
- **Main Chain Block Time**: 20 seconds
- **Average Sidechain Block Time**: 1 second (varies by protocol)
- **Energy Efficiency**: 60-80% improvement over traditional PoW
- **CPU Optimization**: Favors modern general-purpose processors
- **Scalability**: Supports multiple concurrent protocol sidechains