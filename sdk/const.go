// Package sdk is a software development kit for building blockchain applications.
// File sdk/const.go - Constants for the blockchain
package sdk

const (
	// Blockchain Identification
	BlockchainName          = "Go Basic Blockchain"
	BlockchainSymbol        = "GBB"
	BlockchainVersion       = "0.1.0"
	BlockhainOrganizationID = 1 // 1 is reserved for this blockchain "Go Basic Blockchain"
	BlockchainAdminUserID   = 1
	BlockchainAppID         = 1 // 1 is reserved for this blockchain's Core
	BlockchainDevAssetID    = 1
	BlockchainMinerAssetID  = 2

	// Blockchain Parameters
	blockTimeInSec        = 20
	proofOfWorkDifficulty = 2
	// genesisDifficulty is the difficulty stamped on the genesis block.
	genesisDifficulty = 1
	// minAcceptableDifficulty is the lowest difficulty a block may declare.
	// Without a floor, a peer could present a long branch of trivially mined
	// blocks; fork choice weighs work, so cheap blocks must be refused outright.
	minAcceptableDifficulty = 1
	// maxReorgDepth bounds how far back a reorganisation may rewrite history.
	maxReorgDepth     = 100
	transactionFee    = 0.05    // 5 hundredths of a coin (a nickel-ish)
	minTransactionFee = 0.01    // Minimum transaction fee
	minerRewardPCT    = 50.0    // Miner reward is 50% of the transaction fee
	devRewardPCT      = 50.0    // Developer reward is 50% of the transaction fee
	MaxBlockSize      = 1000000 // Maximum block size in bytes (1MB)
	MaxMessageLength  = 4096    // Maximum MESSAGE protocol body length in bytes
	indexCacheSize    = 65536   // Size of the block/transaction index cache (1,572,864 bytes or 1.5 MB)

	// Token Related
	tokenCount       = 33554432
	tokenPrice       = 0.01 // Price of a token in USD
	allowNewTokens   = false
	fundWalletAmount = 100.0 // Default amount to fund new wallets

	// Network Settings
	apiHostname = ":8100"
	p2pHostname = ":8101"

	// Default Addresses
	minerAddress = "MINER" // Will be supplied by the environment
	devAddress   = "DEV"   // Will be supplied by the genesis block

	// Data Storage
	dataFolder   = "data"
	walletFolder = dataFolder + "/wallets"
	blockFolder  = dataFolder + "/blocks"
	cfgFile      = "../../.local.env"

	// Email Settings
	//nolint:unused
	gmailEmail = "" // Currently unused but kept for potential future use
	//nolint:unused
	gmailPassword = "" // Currently unused but kept for potential future use

	// Feature Flags
	EnableAPI = true
	verbose   = true

	// Cryptographic Constants
	saltSize = 32
	// gcmNonceSize is the AES-GCM nonce length in bytes.
	gcmNonceSize = 12
	// maxMiningNonce bounds the simple proof-of-work search. This used to be
	// `maxNonce = 12`, the *AES-GCM nonce size*, so simple PoW gave up after 12
	// attempts and returned an unmined block that was persisted anyway.
	maxMiningNonce = 1 << 32

	// Formatting
	logDateTimeFormat = "2006-01-02 15:04:05"

	// Protocol Versions
	TransactionProtocolVersion = "1.0"
)

// Protocol IDs
const (
	PersistProtocolID  = "PERSIST"
	BankProtocolID     = "BANK"
	MessageProtocolID  = "MESSAGE"
	CoinbaseProtocolID = "COINBASE"
	ChainProtocolID    = "CHAIN"
	// P2PProtocolID is used for node-to-node control messages. It was missing from
	// AvailableProtocols, so every P2P control transaction failed validation.
	P2PProtocolID = "P2P"
	// AnchorProtocolID commits a publisher's sidechain history to the main chain.
	// It is what makes a sidechain more than a private database: see
	// sdk/sidechain_anchor.go.
	AnchorProtocolID = "ANCHOR"
)

// AvailableProtocols is a list of all available protocols
var AvailableProtocols = []string{
	CoinbaseProtocolID,
	BankProtocolID,
	MessageProtocolID,
	PersistProtocolID,
	ChainProtocolID,
	P2PProtocolID,
	AnchorProtocolID,
}
