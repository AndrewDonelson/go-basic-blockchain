package sdk

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	blockchainVersion string
	blockchainName    string
	blockchainSymbol  string
	blockTime         int
	difficulty        int
	transactionFee    float64
	minerRewardPCT    float64
	minerAddress      string
	devRewardPCT      float64
	devAddress        string
	apiHostname       string
	enableAPI         bool
	fundWalletAmount  float64
}

func NewConfig() *Config {
	// step 1: create a new actual config
	cfg := &Config{}

	// step 2: set the default values from the constants
	cfg.blockchainName = BlockchainName
	cfg.blockchainSymbol = BlockchainSymbol
	cfg.blockTime = blockTimeInSec
	cfg.difficulty = proofOfWorkDifficulty
	cfg.transactionFee = transactionFee
	cfg.minerRewardPCT = minerRewardPCT
	cfg.minerAddress = minerAddress
	cfg.devRewardPCT = devRewardPCT
	cfg.devAddress = devAddress
	cfg.apiHostname = apiHostname
	cfg.enableAPI = EnableAPI
	cfg.fundWalletAmount = fundWalletAmount

	//  step 3: Load all values in the .env file if it exists
	err := godotenv.Load(cfgFile)
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// step 4: set the values from the environment variables

	if os.Getenv("BLOCKCHAIN_NAME") != "" {
		if os.Getenv("BLOCKCHAIN_NAME") == BlockchainName {
			fmt.Printf("Warning: Environment BLOCKCHAIN_NAME is set to the default value of %s and will be ignored\n", BlockchainName)
		} else {
			fmt.Printf("Notice: Environment BLOCKCHAIN_NAME is set to %s\n", os.Getenv("BLOCKCHAIN_NAME"))
			cfg.blockchainName = os.Getenv("BLOCKCHAIN_NAME")
		}
	}

	if os.Getenv("BLOCKCHAIN_SYMBOL") != "" {
		if os.Getenv("BLOCKCHAIN_SYMBOL") == BlockchainSymbol {
			fmt.Printf("Warning: Environment BLOCKCHAIN_SYMBOL is set to the default value of %s and will be ignored\n", BlockchainSymbol)
		} else {
			fmt.Printf("Notice: Environment BLOCKCHAIN_SYMBOL is set to %s\n", os.Getenv("BLOCKCHAIN_SYMBOL"))
			cfg.blockchainSymbol = os.Getenv("BLOCKCHAIN_SYMBOL")
		}
	}

	if os.Getenv("BLOCK_TIME") != "" {
		if os.Getenv("BLOCK_TIME") == strconv.Itoa(blockTimeInSec) {
			fmt.Printf("Warning: Environment BLOCK_TIME is set to the default value of %d and will be ignored\n", blockTimeInSec)
		} else {
			fmt.Printf("Notice: Environment BLOCK_TIME is set to %s\n", os.Getenv("BLOCK_TIME"))
			cfg.blockTime = cfg.getIntEnv("BLOCK_TIME", blockTimeInSec)
		}
	}

	if os.Getenv("DIFFICULTY") != "" {
		if os.Getenv("DIFFICULTY") == strconv.Itoa(proofOfWorkDifficulty) {
			fmt.Printf("Warning: Environment DIFFICULTY is set to the default value of %d and will be ignored\n", proofOfWorkDifficulty)
		} else {
			fmt.Printf("Notice: Environment DIFFICULTY is set to %s\n", os.Getenv("DIFFICULTY"))
			cfg.difficulty = cfg.getIntEnv("DIFFICULTY", proofOfWorkDifficulty)
		}
	}

	if os.Getenv("TRANSACTION_FEE") != "" {
		if os.Getenv("TRANSACTION_FEE") == fmt.Sprintf("%.2f", transactionFee) {
			fmt.Printf("Warning: Environment TRANSACTION_FEE is set to the default value of %.2f and will be ignored\n", transactionFee)
		} else {
			fmt.Printf("Notice: Environment TRANSACTION_FEE is set to %s\n", os.Getenv("TRANSACTION_FEE"))
			cfg.transactionFee = cfg.getFloatEnv("TRANSACTION_FEE", transactionFee)
		}
	}

	if os.Getenv("MINER_REWARD_PCT") != "" {
		if os.Getenv("MINER_REWARD_PCT") == fmt.Sprintf("%.2f", minerRewardPCT) {
			fmt.Printf("Warning: Environment MINER_REWARD_PCT is set to the default value of %.2f and will be ignored\n", minerRewardPCT)
		} else {
			fmt.Printf("Notice: Environment MINER_REWARD_PCT is set to %s\n", os.Getenv("MINER_REWARD_PCT"))
			cfg.minerRewardPCT = cfg.getFloatEnv("MINER_REWARD_PCT", minerRewardPCT)
		}
	}

	if os.Getenv("MINER_ADDRESS") != "" {
		if os.Getenv("MINER_ADDRESS") == minerAddress {
			fmt.Printf("Warning: Environment MINER_ADDRESS is set to the default value of %s and will be ignored\n", minerAddress)
		} else {
			fmt.Printf("Notice: Environment MINER_ADDRESS is set to %s\n", os.Getenv("MINER_ADDRESS"))
			cfg.minerAddress = os.Getenv("MINER_ADDRESS")
		}
	}

	if os.Getenv("DEV_REWARD_PCT") != "" {
		if os.Getenv("DEV_REWARD_PCT") == fmt.Sprintf("%.2f", devRewardPCT) {
			fmt.Printf("Warning: Environment DEV_REWARD_PCT is set to the default value of %.2f and will be ignored\n", devRewardPCT)
		} else {
			fmt.Printf("Notice: Environment DEV_REWARD_PCT is set to %s\n", os.Getenv("DEV_REWARD_PCT"))
			cfg.devRewardPCT = cfg.getFloatEnv("DEV_REWARD_PCT", devRewardPCT)
		}
	}

	if os.Getenv("DEV_ADDRESS") != "" {
		if os.Getenv("DEV_ADDRESS") == devAddress {
			fmt.Printf("Warning: Environment DEV_ADDRESS is set to the default value of %s and will be ignored\n", devAddress)
		} else {
			fmt.Printf("Notice: Environment DEV_ADDRESS is set to %s\n", os.Getenv("DEV_ADDRESS"))
			cfg.devAddress = os.Getenv("DEV_ADDRESS")
		}
	}

	if os.Getenv("API_HOSTNAME") != "" {
		if os.Getenv("API_HOSTNAME") == apiHostname {
			fmt.Printf("Warning: Environment API_HOSTNAME is set to the default value of %s and will be ignored\n", apiHostname)
		} else {
			fmt.Printf("Notice: Environment API_HOSTNAME is set to %s\n", os.Getenv("API_HOSTNAME"))
			cfg.apiHostname = os.Getenv("API_HOSTNAME")
		}
	}

	if os.Getenv("ENABLE_API") != "" {
		if os.Getenv("ENABLE_API") == strconv.FormatBool(EnableAPI) {
			fmt.Printf("Warning: Environment ENABLE_API is set to the default value of %t and will be ignored\n", EnableAPI)
		} else {
			fmt.Printf("Notice: Environment ENABLE_API is set to %s\n", os.Getenv("ENABLE_API"))
			cfg.enableAPI = cfg.getBoolEnv("ENABLE_API", EnableAPI)
		}
	}

	if os.Getenv("FUND_WALLET_AMOUNT") != "" {
		if os.Getenv("FUND_WALLET_AMOUNT") == fmt.Sprintf("%.2f", fundWalletAmount) {
			fmt.Printf("Warning: Environment FUND_WALLET_AMOUNT is set to the default value of %.2f and will be ignored\n", fundWalletAmount)
		} else {
			fmt.Printf("Notice: Environment FUND_WALLET_AMOUNT is set to %s\n", os.Getenv("FUND_WALLET_AMOUNT"))
			cfg.fundWalletAmount = cfg.getFloatEnv("FUND_WALLET_AMOUNT", fundWalletAmount)
		}
	}

	// step 5: display config values that will be used
	fmt.Println("Using these Configuration Values:")
	fmt.Printf("- Blockchain Name: %s\n", cfg.blockchainName)
	fmt.Printf("- Blockchain Symbol: %s\n", cfg.blockchainSymbol)
	fmt.Printf("- Block Time: %d seconds\n", cfg.blockTime)
	fmt.Printf("- Difficulty: %d\n", cfg.difficulty)
	fmt.Printf("- Transaction Fee: %.2f\n", cfg.transactionFee)
	fmt.Printf("- Miner Reward Percentage: %.2f%%\n", cfg.minerRewardPCT)
	fmt.Printf("- Miner Address: %s\n", cfg.minerAddress)
	fmt.Printf("- Developer Reward Percentage: %.2f%%\n", cfg.devRewardPCT)
	fmt.Printf("- Developer Address: %s\n", cfg.devAddress)
	fmt.Printf("- API Hostname: %s\n", cfg.apiHostname)
	fmt.Printf("- Enable API: %v\n", cfg.enableAPI)
	fmt.Printf("- Fund Wallet Amount: %.2f\n", cfg.fundWalletAmount)

	return cfg

}

func (c *Config) getIntEnv(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Invalid value for environment variable %s: %s\n", key, valueStr)
		return defaultValue
	}

	return value
}

func (c *Config) getFloatEnv(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		log.Printf("Invalid value for environment variable %s: %s\n", key, valueStr)
		return defaultValue
	}

	return value
}

func (c *Config) getBoolEnv(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		log.Printf("Invalid value for environment variable %s: %s\n", key, valueStr)
		return defaultValue
	}

	return value
}
