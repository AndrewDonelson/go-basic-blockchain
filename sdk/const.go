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
	blockTimeInSec        = 5
	proofOfWorkDifficulty = 4
	transactionFee        = 0.05    // 5 hundredths of a coin (a nickel-ish)
	minTransactionFee     = 0.01    // Minimum transaction fee
	minerRewardPCT        = 50.0    // Miner reward is 50% of the transaction fee
	devRewardPCT          = 50.0    // Developer reward is 50% of the transaction fee
	MaxBlockSize          = 1000000 // Maximum block size in bytes (1MB)
	indexCacheSize        = 65536   // Size of the block/transaction index cache (1,572,864 bytes or 1.5 MB)

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
	dataFolder   = "../data"
	walletFolder = dataFolder + "/wallets"
	blockFolder  = dataFolder + "/blocks"
	cfgFile      = "../../.local.env"

	// Email Settings
	gmailEmail    = ""
	gmailPassword = "" // Should be supplied by the environment

	// Feature Flags
	EnableAPI = true
	verbose   = true

	// Cryptographic Constants
	saltSize = 32
	maxNonce = 12 // bytes

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
)

// AvailableProtocols is a list of all available protocols
var AvailableProtocols = []string{
	CoinbaseProtocolID,
	BankProtocolID,
	MessageProtocolID,
	PersistProtocolID,
	ChainProtocolID,
}
