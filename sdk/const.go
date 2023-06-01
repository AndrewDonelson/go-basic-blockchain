package sdk

import "math"

const (
	// TODO:
	//	- Move these to a config file (most of them)
	//  - Move the a few into the genesis block (minerAddress, devAddress)

	// BlockchainVersion is the version of the blockchain
	BlockchainVersion = "0.1.0"

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

	// miner reward is 50% of the transaction fee
	minerRewardPCT = 50.0

	// minerAddress is the address of the miner (will be supplied by the environment)
	minerAddress = "MINER"

	// devreward is 50% of the transaction fee
	devRewardPCT = 50.0

	// devAddress is the address of the developer
	devAddress = "DEV" // will be supplied by the genesis block

	// TransactionProtocolVersion is the Tranasction Protocol Version
	TransactionProtocolVersion = "1.0"

	// BankProtocolID is the Bank Protocol ID
	BankProtocolID = "BANK"

	// MessageProtocolID is the Message Protocol ID
	MessageProtocolID = "MESSAGE"

	// CoinbaseProtocolID is the Coinbase Protocol ID
	CoinbaseProtocolID = "COINBASE"

	// AvailableProtocols is the list of available protocols
	AvailableProtocols = BankProtocolID + "," + MessageProtocolID + "," + CoinbaseProtocolID

	// saltSize is 16 bytes
	saltSize = 16

	// default Amount to fund new wallets is 100 coins
	fundWalletAmount = 100.0

	// data folder is the folder where the blockchain data is stored
	dataFolder = "../data"

	// Log Date/Time format
	logDateTimeFormat = "2006-01-02 15:04:05"

	// EnableAPI is a flag to enable/disable the API
	EnableAPI = true

	// maxNonce is the maximum value for a nonce
	maxNonce = math.MaxInt64
)