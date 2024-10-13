// Package sdk is a software development kit for building blockchain applications.
// File sdk/const.go - Constants for the blockchain
package sdk

const (
	// Blockchain Identification
	BlockchainName           = "Go Basic Blockchain"
	BlockchainSymbol         = "GBB"
	BlockchainVersion        = "0.2.0"
	BlockchainOrganizationID = 1 // 1 is reserved for this blockchain "Go Basic Blockchain"
	BlockchainAdminUserID    = 1
	BlockchainAppID          = 1 // 1 is reserved for this blockchain's Core
	BlockchainDevAssetID     = 1
	BlockchainMinerAssetID   = 2

	// Blockchain Parameters
	BlockTimeInSec        = 5
	ProofOfWorkDifficulty = 4
	TransactionFee        = 0.05    // 5 hundredths of a coin (a nickel-ish)
	MinTransactionFee     = 0.01    // Minimum transaction fee
	MinerRewardPCT        = 50.0    // Miner reward is 50% of the transaction fee
	DevRewardPCT          = 50.0    // Developer reward is 50% of the transaction fee
	MaxBlockSize          = 1000000 // Maximum block size in bytes (1MB)
	IndexCacheSize        = 65536   // Size of the block/transaction index cache (1,572,864 bytes or 1.5 MB)

	// Token Related
	TokenCount       = 33554432
	TokenPrice       = 0.01 // Price of a token in USD
	AllowNewTokens   = false
	FundWalletAmount = 100.0 // Default amount to fund new wallets

	// Network Settings
	APIHostname = ":8100"
	P2PHostname = ":8101"

	// Default Addresses
	MinerAddress = "MINER" // Will be supplied by the environment
	DevAddress   = "DEV"   // Will be supplied by the genesis block

	// Data Storage
	DataFolder   = "../data"
	WalletFolder = DataFolder + "/wallets"
	BlockFolder  = DataFolder + "/blocks"
	ConfigFile   = "../../.local.env"

	// Email Settings
	GmailEmail    = ""
	GmailPassword = "" // Should be supplied by the environment

	// Feature Flags
	EnableAPI = true
	Verbose   = true

	// Cryptographic Constants
	SaltSize = 32
	MaxNonce = 12 // bytes

	// Formatting
	LogDateTimeFormat = "2006-01-02 15:04:05"

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
