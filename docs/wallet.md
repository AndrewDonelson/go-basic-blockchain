# Wallet Guide

Complete guide to wallet creation, management, and security in the Go Basic Blockchain.

## 🎯 Overview

Wallets in the Go Basic Blockchain provide secure storage for private keys and enable transaction signing. Each wallet is encrypted with a strong passphrase and supports multiple transaction types.

## 🔐 Security Features

### Encryption Standards

**AES-GCM Encryption**:
- 256-bit key encryption
- Galois/Counter Mode for authenticated encryption
- Protection against tampering and forgery

**Scrypt Key Derivation**:
- Memory-hard key derivation function
- Configurable parameters for security vs performance
- Salt-based protection against rainbow table attacks

**Password Requirements**:
- Minimum 12 characters
- Mix of uppercase, lowercase, numbers, symbols
- No common patterns or dictionary words

### Key Management

**Private Key Generation**:
- Cryptographically secure random generation
- Elliptic curve cryptography (secp256k1)
- Deterministic key derivation

**Public Key Derivation**:
- Derived from private key using elliptic curve
- Compressed and uncompressed formats
- Address generation from public key

## 💼 Wallet Creation

### Via Web Interface

1. **Navigate to Wallet Section**:
   - Open `http://localhost:8200`
   - Click "Create Wallet" button

2. **Enter Strong Passphrase**:
   - Use at least 12 characters
   - Include uppercase, lowercase, numbers, symbols
   - Avoid common patterns

3. **Save Wallet File**:
   - Download encrypted wallet file
   - Store in secure location
   - Backup multiple copies

### Via API

**Create Wallet**:
```bash
curl -X POST http://localhost:8200/api/wallet/create \
  -H "Content-Type: application/json" \
  -d '{"passphrase": "your-strong-passphrase"}'
```

**Response**:
```json
{
  "address": "wallet123abc...",
  "public_key": "04abc123...",
  "encrypted": true,
  "created_at": 1640995200
}
```

### Via Code

```go
import "github.com/yourusername/go-basic-blockchain/sdk"

// Create new wallet
wallet, err := sdk.CreateWallet("your-strong-passphrase")
if err != nil {
    log.Fatal(err)
}

// Get wallet address
address := wallet.GetAddress()
fmt.Printf("Wallet address: %s\n", address)
```

## 🔑 Wallet Operations

### Opening a Wallet

**Via Web Interface**:
1. Click "Open Wallet"
2. Upload wallet file
3. Enter passphrase
4. Access wallet functions

**Via API**:
```bash
curl -X POST http://localhost:8200/api/wallet/open \
  -H "Content-Type: application/json" \
  -d '{
    "wallet_data": "encrypted-wallet-data...",
    "passphrase": "your-passphrase"
  }'
```

**Via Code**:
```go
// Open existing wallet
wallet, err := sdk.OpenWallet(walletData, passphrase)
if err != nil {
    log.Fatal(err)
}
```

### Getting Balance

**Via Web Interface**:
- Balance displayed in wallet dashboard
- Real-time updates
- Transaction history

**Via API**:
```bash
curl http://localhost:8200/api/wallet/balance/wallet123abc...
```

**Response**:
```json
{
  "address": "wallet123abc...",
  "balance": 150.75,
  "confirmed_balance": 150.75,
  "pending_balance": 0,
  "last_updated": 1640995200
}
```

**Via Code**:
```go
balance := wallet.GetBalance()
fmt.Printf("Balance: %.2f\n", balance)
```

### Transaction History

**Via API**:
```bash
curl "http://localhost:8200/api/wallet/transactions/wallet123abc...?limit=20"
```

**Response**:
```json
{
  "address": "wallet123abc...",
  "transactions": [
    {
      "id": "tx123...",
      "type": "BANK",
      "from": "wallet1...",
      "to": "wallet2...",
      "amount": 10.5,
      "timestamp": 1640995190,
      "block_height": 1234,
      "confirmed": true
    }
  ],
  "total": 45
}
```

## 💸 Creating Transactions

### Bank Transaction (Transfer Coins)

**Via Web Interface**:
1. Click "Send Coins"
2. Enter recipient address
3. Enter amount
4. Enter passphrase
5. Click "Send"

**Via API**:
```bash
curl -X POST http://localhost:8200/api/transaction/create \
  -H "Content-Type: application/json" \
  -d '{
    "type": "BANK",
    "from": "wallet123abc...",
    "to": "wallet456def...",
    "amount": 10.5,
    "passphrase": "your-passphrase"
  }'
```

**Via Code**:
```go
// Create bank transaction
tx := &sdk.BankTransaction{
    From:   wallet.GetAddress(),
    To:     recipientAddress,
    Amount: 10.5,
}

// Sign transaction
signedTx, err := wallet.SignTransaction(tx, passphrase)
if err != nil {
    log.Fatal(err)
}

// Broadcast transaction
err = blockchain.AddTransaction(signedTx)
if err != nil {
    log.Fatal(err)
}
```

### Message Transaction (Encrypted Message)

**Via API**:
```bash
curl -X POST http://localhost:8200/api/transaction/create \
  -H "Content-Type: application/json" \
  -d '{
    "type": "MESSAGE",
    "from": "wallet123abc...",
    "to": "wallet456def...",
    "message": "Hello, blockchain!",
    "passphrase": "your-passphrase"
  }'
```

**Via Code**:
```go
// Create message transaction
tx := &sdk.MessageTransaction{
    From:    wallet.GetAddress(),
    To:      recipientAddress,
    Message: "Hello, blockchain!",
}

// Sign and broadcast
signedTx, err := wallet.SignTransaction(tx, passphrase)
if err != nil {
    log.Fatal(err)
}

err = blockchain.AddTransaction(signedTx)
if err != nil {
    log.Fatal(err)
}
```

## 🔒 Security Best Practices

### Passphrase Security

**Strong Passphrase Requirements**:
- Minimum 12 characters
- Mix of character types
- No common patterns
- Unique for each wallet

**Passphrase Examples**:
```
✅ Good: "MySecureWallet2024!@#"
✅ Good: "K9#mN2$pL8@vX5&qR7"
❌ Bad: "password123"
❌ Bad: "123456789"
❌ Bad: "qwertyuiop"
```

### Wallet File Security

**Storage Recommendations**:
- Encrypted external drive
- Multiple secure backups
- Offline storage
- Regular backup updates

**Backup Strategy**:
- Primary backup on encrypted drive
- Secondary backup in secure cloud
- Tertiary backup in safe deposit box
- Regular backup verification

### Network Security

**API Security**:
- Use HTTPS in production
- Rotate API keys regularly
- Monitor for suspicious activity
- Implement rate limiting

**Transaction Security**:
- Verify recipient addresses
- Double-check amounts
- Use secure connections
- Monitor transaction confirmations

## 🛠️ Advanced Features

### Multi-Signature Support

**Creating Multi-Sig Wallet**:
```go
// Create multi-signature wallet
multiSig := sdk.CreateMultiSigWallet([]string{
    "wallet1...",
    "wallet2...",
    "wallet3...",
}, 2) // Require 2 of 3 signatures
```

**Signing Multi-Sig Transaction**:
```go
// Sign with first wallet
signed1 := wallet1.SignMultiSigTransaction(tx, passphrase1)

// Sign with second wallet
signed2 := wallet2.SignMultiSigTransaction(tx, passphrase2)

// Combine signatures
finalTx := multiSig.CombineSignatures(signed1, signed2)
```

### Hardware Wallet Integration

**Supported Hardware**:
- Ledger Nano S/X
- Trezor Model T
- KeepKey

**Integration Code**:
```go
// Connect to hardware wallet
hw := sdk.ConnectHardwareWallet("ledger")

// Get address
address := hw.GetAddress()

// Sign transaction
signedTx := hw.SignTransaction(tx)
```

### Watch-Only Wallets

**Creating Watch-Only Wallet**:
```go
// Create watch-only wallet from public key
watchWallet := sdk.CreateWatchOnlyWallet(publicKey)

// Monitor transactions
transactions := watchWallet.GetTransactions()
```

## 🔧 Troubleshooting

### Common Issues

**"Invalid Passphrase" Error**:
- Check for typos
- Verify caps lock status
- Try copy-paste from secure note
- Check for extra spaces

**"Wallet Not Found" Error**:
- Verify wallet file path
- Check file permissions
- Ensure wallet file is not corrupted
- Try importing wallet again

**"Insufficient Balance" Error**:
- Check confirmed vs pending balance
- Account for transaction fees
- Wait for pending transactions to confirm
- Verify transaction amounts

**"Transaction Failed" Error**:
- Check network connectivity
- Verify recipient address
- Ensure sufficient balance
- Check transaction fee

### Recovery Procedures

**Lost Passphrase**:
- No recovery possible
- Create new wallet
- Transfer any remaining funds
- Update backup strategy

**Corrupted Wallet File**:
- Try backup copies
- Use wallet recovery tools
- Contact support if needed
- Create new wallet if necessary

**Stolen Wallet**:
- Immediately create new wallet
- Transfer funds to new wallet
- Report incident
- Review security practices

## 📊 Wallet Statistics

### Performance Metrics

**Creation Time**:
- Standard wallet: <1 second
- Multi-sig wallet: <2 seconds
- Hardware wallet: <5 seconds

**Transaction Signing**:
- Bank transaction: <100ms
- Message transaction: <200ms
- Multi-sig transaction: <500ms

**Memory Usage**:
- Standard wallet: 1MB
- Multi-sig wallet: 2MB
- Hardware wallet: 5MB

### Security Metrics

**Encryption Strength**:
- AES-256-GCM: 256-bit key
- Scrypt: N=1048576 (production)
- Scrypt: N=16384 (testing)

**Key Derivation**:
- Salt: 32 bytes
- Iterations: 1,048,576
- Memory: 8MB

## 🔮 Future Enhancements

### Planned Features

**Advanced Security**:
- Hardware security modules (HSM)
- Threshold signatures
- Zero-knowledge proofs
- Quantum-resistant cryptography

**User Experience**:
- Mobile wallet app
- Web wallet interface
- Desktop wallet application
- Browser extension

**Functionality**:
- Smart contract integration
- DeFi protocol support
- NFT wallet features
- Cross-chain transactions

---

**For more information about wallet security and advanced features, see the [Security](security.md) and [Development](development.md) documentation.** 