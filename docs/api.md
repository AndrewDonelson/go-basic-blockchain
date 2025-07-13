# API Reference

Complete RESTful API documentation for the Go Basic Blockchain. All endpoints return JSON responses and support standard HTTP methods.

## 🔗 Base URL

```
http://localhost:8200/api
```

## 🔐 Authentication

### API Key Authentication

Most endpoints require an API key for authentication. Include the key in the request header:

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8200/api/endpoint
```

### Session Authentication

Web interface endpoints use session-based authentication. Login via the web interface to establish a session.

## 📊 Blockchain Endpoints

### Get Blockchain Status

**Endpoint**: `GET /api/blockchain/status`

**Description**: Get current blockchain status including height, difficulty, and mining status.

**Response**:
```json
{
  "height": 1234,
  "difficulty": 4,
  "mining": true,
  "last_block_hash": "0000abc123...",
  "total_difficulty": 5678,
  "pending_transactions": 5
}
```

**Example**:
```bash
curl http://localhost:8200/api/blockchain/status
```

### Get Recent Blocks

**Endpoint**: `GET /api/blockchain/blocks?limit=10`

**Description**: Get recent blocks from the blockchain.

**Parameters**:
- `limit` (optional): Number of blocks to return (default: 10, max: 100)

**Response**:
```json
{
  "blocks": [
    {
      "index": 1234,
      "timestamp": 1640995200,
      "hash": "0000abc123...",
      "previous_hash": "0000def456...",
      "transactions": 5,
      "difficulty": 4,
      "nonce": 12345
    }
  ],
  "total": 1234
}
```

**Example**:
```bash
curl "http://localhost:8200/api/blockchain/blocks?limit=5"
```

### Get Block by Hash

**Endpoint**: `GET /api/blockchain/block/{hash}`

**Description**: Get detailed information about a specific block.

**Response**:
```json
{
  "index": 1234,
  "timestamp": 1640995200,
  "hash": "0000abc123...",
  "previous_hash": "0000def456...",
  "merkle_root": "abc123...",
  "difficulty": 4,
  "nonce": 12345,
  "transactions": [
    {
      "id": "tx123...",
      "type": "BANK",
      "from": "wallet1...",
      "to": "wallet2...",
      "amount": 10.5,
      "timestamp": 1640995190
    }
  ]
}
```

**Example**:
```bash
curl http://localhost:8200/api/blockchain/block/0000abc123...
```

## 💰 Wallet Endpoints

### Create Wallet

**Endpoint**: `POST /api/wallet/create`

**Description**: Create a new encrypted wallet.

**Request Body**:
```json
{
  "passphrase": "your-strong-passphrase"
}
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

**Example**:
```bash
curl -X POST http://localhost:8200/api/wallet/create \
  -H "Content-Type: application/json" \
  -d '{"passphrase": "your-strong-passphrase"}'
```

### Get Wallet Balance

**Endpoint**: `GET /api/wallet/balance/{address}`

**Description**: Get the current balance of a wallet.

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

**Example**:
```bash
curl http://localhost:8200/api/wallet/balance/wallet123abc...
```

### Get Wallet Transactions

**Endpoint**: `GET /api/wallet/transactions/{address}?limit=20`

**Description**: Get transaction history for a wallet.

**Parameters**:
- `limit` (optional): Number of transactions to return (default: 20, max: 100)

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

**Example**:
```bash
curl "http://localhost:8200/api/wallet/transactions/wallet123abc...?limit=10"
```

### Import Wallet

**Endpoint**: `POST /api/wallet/import`

**Description**: Import an existing wallet from file.

**Request Body**:
```json
{
  "wallet_data": "encrypted-wallet-data...",
  "passphrase": "your-passphrase"
}
```

**Response**:
```json
{
  "address": "wallet123abc...",
  "imported": true,
  "created_at": 1640995200
}
```

**Example**:
```bash
curl -X POST http://localhost:8200/api/wallet/import \
  -H "Content-Type: application/json" \
  -d '{
    "wallet_data": "encrypted-wallet-data...",
    "passphrase": "your-passphrase"
  }'
```

## 💸 Transaction Endpoints

### Create Transaction

**Endpoint**: `POST /api/transaction/create`

**Description**: Create and broadcast a new transaction.

**Request Body**:
```json
{
  "type": "BANK",
  "from": "wallet123abc...",
  "to": "wallet456def...",
  "amount": 10.5,
  "passphrase": "your-passphrase"
}
```

**Response**:
```json
{
  "transaction_id": "tx123...",
  "status": "pending",
  "created_at": 1640995200,
  "fee": 0.001
}
```

**Example**:
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

### Create Message Transaction

**Endpoint**: `POST /api/transaction/create`

**Description**: Create an encrypted message transaction.

**Request Body**:
```json
{
  "type": "MESSAGE",
  "from": "wallet123abc...",
  "to": "wallet456def...",
  "message": "Hello, blockchain!",
  "passphrase": "your-passphrase"
}
```

**Response**:
```json
{
  "transaction_id": "tx123...",
  "status": "pending",
  "created_at": 1640995200,
  "encrypted": true
}
```

**Example**:
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

### Get Transaction Status

**Endpoint**: `GET /api/transaction/{transaction_id}`

**Description**: Get the status and details of a transaction.

**Response**:
```json
{
  "id": "tx123...",
  "type": "BANK",
  "from": "wallet123abc...",
  "to": "wallet456def...",
  "amount": 10.5,
  "status": "confirmed",
  "block_height": 1234,
  "confirmations": 6,
  "timestamp": 1640995190,
  "fee": 0.001
}
```

**Example**:
```bash
curl http://localhost:8200/api/transaction/tx123...
```

### Get Pending Transactions

**Endpoint**: `GET /api/transaction/pending?limit=20`

**Description**: Get pending transactions in the mempool.

**Parameters**:
- `limit` (optional): Number of transactions to return (default: 20, max: 100)

**Response**:
```json
{
  "transactions": [
    {
      "id": "tx123...",
      "type": "BANK",
      "from": "wallet1...",
      "to": "wallet2...",
      "amount": 10.5,
      "timestamp": 1640995190,
      "fee": 0.001
    }
  ],
  "total": 5
}
```

**Example**:
```bash
curl "http://localhost:8200/api/transaction/pending?limit=10"
```

## ⛏️ Mining Endpoints

### Start Mining

**Endpoint**: `POST /api/mining/start`

**Description**: Start the mining process.

**Response**:
```json
{
  "status": "started",
  "difficulty": 4,
  "target": "0000...",
  "started_at": 1640995200
}
```

**Example**:
```bash
curl -X POST http://localhost:8200/api/mining/start
```

### Stop Mining

**Endpoint**: `POST /api/mining/stop`

**Description**: Stop the mining process.

**Response**:
```json
{
  "status": "stopped",
  "blocks_mined": 123,
  "stopped_at": 1640995200
}
```

**Example**:
```bash
curl -X POST http://localhost:8200/api/mining/stop
```

### Get Mining Status

**Endpoint**: `GET /api/mining/status`

**Description**: Get current mining status and statistics.

**Response**:
```json
{
  "mining": true,
  "difficulty": 4,
  "target": "0000...",
  "hash_rate": 1000,
  "blocks_mined": 123,
  "started_at": 1640995200
}
```

**Example**:
```bash
curl http://localhost:8200/api/mining/status
```

## 🌐 Network Endpoints

### Get Network Status

**Endpoint**: `GET /api/network/status`

**Description**: Get network status and peer information.

**Response**:
```json
{
  "peers": 5,
  "connections": 3,
  "synced": true,
  "last_sync": 1640995200,
  "network_difficulty": 4
}
```

**Example**:
```bash
curl http://localhost:8200/api/network/status
```

### Get Connected Peers

**Endpoint**: `GET /api/network/peers`

**Description**: Get list of connected peers.

**Response**:
```json
{
  "peers": [
    {
      "address": "192.168.1.100:8100",
      "version": "1.0.0",
      "last_seen": 1640995200,
      "synced": true
    }
  ],
  "total": 3
}
```

**Example**:
```bash
curl http://localhost:8200/api/network/peers
```

### Add Peer

**Endpoint**: `POST /api/network/peer`

**Description**: Manually add a peer to the network.

**Request Body**:
```json
{
  "address": "192.168.1.100:8100"
}
```

**Response**:
```json
{
  "address": "192.168.1.100:8100",
  "added": true,
  "connected": true
}
```

**Example**:
```bash
curl -X POST http://localhost:8200/api/network/peer \
  -H "Content-Type: application/json" \
  -d '{"address": "192.168.1.100:8100"}'
```

## 🔧 Configuration Endpoints

### Get Configuration

**Endpoint**: `GET /api/config`

**Description**: Get current blockchain configuration.

**Response**:
```json
{
  "mining_difficulty": 4,
  "block_reward": 50,
  "block_time": 10,
  "max_transactions_per_block": 1000,
  "network_port": 8100,
  "api_port": 8200
}
```

**Example**:
```bash
curl http://localhost:8200/api/config
```

### Update Configuration

**Endpoint**: `PUT /api/config`

**Description**: Update blockchain configuration (requires restart).

**Request Body**:
```json
{
  "mining_difficulty": 5,
  "block_reward": 25,
  "block_time": 15
}
```

**Response**:
```json
{
  "updated": true,
  "restart_required": true,
  "message": "Configuration updated. Restart required."
}
```

**Example**:
```bash
curl -X PUT http://localhost:8200/api/config \
  -H "Content-Type: application/json" \
  -d '{
    "mining_difficulty": 5,
    "block_reward": 25,
    "block_time": 15
  }'
```

## 📊 Statistics Endpoints

### Get Blockchain Statistics

**Endpoint**: `GET /api/stats/blockchain`

**Description**: Get comprehensive blockchain statistics.

**Response**:
```json
{
  "total_blocks": 1234,
  "total_transactions": 5678,
  "total_wallets": 100,
  "total_coins": 50000,
  "average_block_time": 10.5,
  "average_transactions_per_block": 4.6,
  "network_hash_rate": 1000,
  "uptime": 86400
}
```

**Example**:
```bash
curl http://localhost:8200/api/stats/blockchain
```

### Get Mining Statistics

**Endpoint**: `GET /api/stats/mining`

**Description**: Get mining performance statistics.

**Response**:
```json
{
  "blocks_mined": 123,
  "total_rewards": 6150,
  "average_mining_time": 10.2,
  "hash_rate": 1000,
  "difficulty_changes": 5,
  "mining_efficiency": 0.95
}
```

**Example**:
```bash
curl http://localhost:8200/api/stats/mining
```

## ❌ Error Responses

All endpoints may return error responses with the following format:

```json
{
  "error": "Error message",
  "code": 400,
  "details": "Additional error details"
}
```

### Common Error Codes

- `400 Bad Request`: Invalid request format or parameters
- `401 Unauthorized`: Missing or invalid authentication
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `409 Conflict`: Resource conflict (e.g., insufficient balance)
- `422 Unprocessable Entity`: Validation error
- `500 Internal Server Error`: Server error

### Example Error Response

```bash
curl http://localhost:8200/api/wallet/balance/invalid-address
```

**Response**:
```json
{
  "error": "Wallet not found",
  "code": 404,
  "details": "Address 'invalid-address' does not exist"
}
```

## 📝 Rate Limiting

API endpoints are subject to rate limiting to prevent abuse:

- **Authentication endpoints**: 10 requests per minute
- **Read endpoints**: 100 requests per minute
- **Write endpoints**: 20 requests per minute
- **Mining endpoints**: 5 requests per minute

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1640995260
```

## 🔒 Security Considerations

### API Key Security

- Store API keys securely
- Use HTTPS in production
- Rotate keys regularly
- Monitor API usage

### Input Validation

- All inputs are validated
- Sanitize user inputs
- Use parameterized queries
- Validate file uploads

### Error Handling

- Don't expose internal errors
- Log security events
- Monitor for suspicious activity
- Implement proper CORS

---

**For more information about specific features, see the [Wallet Guide](wallet.md) and [Helios Consensus](helios.md) documentation.** 