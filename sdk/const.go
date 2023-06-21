// Package sdk is a software development kit for building blockchain applications.
// File sdk/const.go - Constants for the blockchain
package sdk

const (

	// BlockchainName is the name of the blockchain
	BlockchainName = "Go Basic Blockchain"

	// BlockchainSymbol is the symbol of the blockchain
	BlockchainSymbol = "GBB"

	// block time is 5 seconds
	blockTimeInSec = 5

	// proofOfWorkDifficulty is the number of leading zeros that must be found in the hash of a block
	proofOfWorkDifficulty = 4

	// transaction fee is 5 hunderths of a coin (a nickle-ish)
	transactionFee = 0.05

	// tokenCount is the number of tokens to create
	tokenCount = 33554432

	// tokenPrice is the price of a token is USD
	tokenPrice = 0.01

	// allowNewTokens is a flag to allow/disallow new tokens
	allowNewTokens = false

	// miner reward is 50% of the transaction fee
	minerRewardPCT = 50.0

	// minerAddress is the address of the miner (will be supplied by the environment)
	minerAddress = "MINER"

	// devreward is 50% of the transaction fee
	devRewardPCT = 50.0

	// devAddress is the address of the developer
	devAddress = "DEV" // will be supplied by the genesis block

	// hostname & port for the API
	apiHostname = ":8000"

	// default Amount to fund new wallets is 100 coins
	fundWalletAmount = 100.0

	// data folder is the folder where the blockchain data is stored
	dataFolder = "../data"

	// EnableAPI is a flag to enable/disable the API
	EnableAPI = true

	// verbose is a flag to enable/disable verbose logging
	verbose = false

	/*************************************** Internal Constants ***************************************/

	// BlockchainVersion is the version of the blockchain
	BlockchainVersion = "0.1.0"

	// cfgFolder is the folder where the config file is stored
	cfgFile = "../.env"

	// walletFolder is the folder where the wallets are stored (within the data folder)
	walletFolder = dataFolder + "/wallets"

	// blockFolder is the folder where the blocks are stored (within the data folder)
	blockFolder = dataFolder + "/blocks"

	// Log Date/Time format
	logDateTimeFormat = "2006-01-02 15:04:05"

	// maxNonce is the maximum value for a nonce
	maxNonce = 12 // bytes

	// saltSize is the size of the salt used for hashing
	saltSize = 32

	// indexCacheSize is the size of the block/transaction index cache (1,572,864 bytes or 1.5 MB)
	indexCacheSize = 65536 // 2^16

	// TransactionProtocolVersion is the Tranasction Protocol Version
	TransactionProtocolVersion = "1.0"

	// PersistProtocolID is the Data Persistance Protocol ID
	PersistProtocolID = "PERSIST"

	// BankProtocolID is the Bank Protocol ID
	BankProtocolID = "BANK"

	// MessageProtocolID is the Message Protocol ID
	MessageProtocolID = "MESSAGE"

	// CoinbaseProtocolID is the Coinbase Protocol ID
	CoinbaseProtocolID = "COINBASE"
)

// AvailableProtocols is a list of all available protocols
var AvailableProtocols = []string{
	CoinbaseProtocolID,
	BankProtocolID,
	MessageProtocolID,
	PersistProtocolID,
}
