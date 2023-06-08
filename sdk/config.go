// file: sdk/config.go - The main Config file
// package: sdk
// description: This file contains the Config struct and all the methods associated with it.
package sdk

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	blockchainName   string
	blockchainSymbol string
	blockTime        int
	difficulty       int
	transactionFee   float64
	minerRewardPCT   float64
	minerAddress     string
	devRewardPCT     float64
	devAddress       string
	apiHostname      string
	enableAPI        bool
	fundWalletAmount float64
	promptUpdate     bool // This is used internally to check if user added/changed default value from prompt
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
			//fmt.Printf("Warning: Environment BLOCKCHAIN_NAME is set to the default value of %s and will be ignored\n", BlockchainName)
			cfg.blockchainName = cfg.promptValue("BLOCKCHAIN_NAME", BlockchainName, false, "string").(string)
		} else {
			fmt.Printf("Notice: Environment BLOCKCHAIN_NAME is set to %s\n", os.Getenv("BLOCKCHAIN_NAME"))
			cfg.blockchainName = os.Getenv("BLOCKCHAIN_NAME")
		}
	}

	if os.Getenv("BLOCKCHAIN_SYMBOL") != "" {
		if os.Getenv("BLOCKCHAIN_SYMBOL") == BlockchainSymbol {
			//fmt.Printf("Warning: Environment BLOCKCHAIN_SYMBOL is set to the default value of %s and will be ignored\n", BlockchainSymbol)
			cfg.blockchainSymbol = cfg.promptValue("BLOCKCHAIN_SYMBOL", BlockchainSymbol, false, "string").(string)
		} else {
			fmt.Printf("Notice: Environment BLOCKCHAIN_SYMBOL is set to %s\n", os.Getenv("BLOCKCHAIN_SYMBOL"))
			cfg.blockchainSymbol = os.Getenv("BLOCKCHAIN_SYMBOL")
		}
	}

	if os.Getenv("BLOCK_TIME") != "" {
		if os.Getenv("BLOCK_TIME") == strconv.Itoa(blockTimeInSec) {
			//fmt.Printf("Warning: Environment BLOCK_TIME is set to the default value of %d and will be ignored\n", blockTimeInSec)
			cfg.blockTime = cfg.promptValue("BLOCK_TIME", fmt.Sprintf("%d", blockTimeInSec), false, "int").(int)
		} else {
			fmt.Printf("Notice: Environment BLOCK_TIME is set to %s\n", os.Getenv("BLOCK_TIME"))
			cfg.blockTime = cfg.getIntEnv("BLOCK_TIME", blockTimeInSec)
		}
	}

	if os.Getenv("DIFFICULTY") != "" {
		if os.Getenv("DIFFICULTY") == strconv.Itoa(proofOfWorkDifficulty) {
			//fmt.Printf("Warning: Environment DIFFICULTY is set to the default value of %d and will be ignored\n", proofOfWorkDifficulty)
			cfg.difficulty = cfg.promptValue("DIFFICULTY", fmt.Sprintf("%d", proofOfWorkDifficulty), false, "int").(int)
		} else {
			fmt.Printf("Notice: Environment DIFFICULTY is set to %s\n", os.Getenv("DIFFICULTY"))
			cfg.difficulty = cfg.getIntEnv("DIFFICULTY", proofOfWorkDifficulty)
		}
	}

	if os.Getenv("TRANSACTION_FEE") != "" {
		if os.Getenv("TRANSACTION_FEE") == fmt.Sprintf("%.2f", transactionFee) {
			//fmt.Printf("Warning: Environment TRANSACTION_FEE is set to the default value of %.2f and will be ignored\n", transactionFee)
			cfg.transactionFee = cfg.promptValue("TRANSACTION_FEE", fmt.Sprintf("%.2f", transactionFee), false, "float").(float64)
		} else {
			fmt.Printf("Notice: Environment TRANSACTION_FEE is set to %s\n", os.Getenv("TRANSACTION_FEE"))
			cfg.transactionFee = cfg.getFloatEnv("TRANSACTION_FEE", transactionFee)
		}
	}

	if os.Getenv("MINER_REWARD_PCT") != "" {
		if os.Getenv("MINER_REWARD_PCT") == fmt.Sprintf("%.2f", minerRewardPCT) {
			//fmt.Printf("Warning: Environment MINER_REWARD_PCT is set to the default value of %.2f and will be ignored\n", minerRewardPCT)
			cfg.minerRewardPCT = cfg.promptValue("MINER_REWARD_PCT", fmt.Sprintf("%.2f", minerRewardPCT), false, "float").(float64)
		} else {
			fmt.Printf("Notice: Environment MINER_REWARD_PCT is set to %s\n", os.Getenv("MINER_REWARD_PCT"))
			cfg.minerRewardPCT = cfg.getFloatEnv("MINER_REWARD_PCT", minerRewardPCT)
		}
	}

	if os.Getenv("MINER_ADDRESS") != "" {
		if os.Getenv("MINER_ADDRESS") == minerAddress {
			//fmt.Printf("Warning: Environment MINER_ADDRESS is set to the default value of %s and will be ignored\n", minerAddress)
			cfg.minerAddress = cfg.promptValue("MINER_ADDRESS", minerAddress, true, "string").(string)
		} else {
			fmt.Printf("Notice: Environment MINER_ADDRESS is set to %s\n", os.Getenv("MINER_ADDRESS"))
			cfg.minerAddress = os.Getenv("MINER_ADDRESS")
		}
	}

	if os.Getenv("DEV_REWARD_PCT") != "" {
		if os.Getenv("DEV_REWARD_PCT") == fmt.Sprintf("%.2f", devRewardPCT) {
			//fmt.Printf("Warning: Environment DEV_REWARD_PCT is set to the default value of %.2f and will be ignored\n", devRewardPCT)
			cfg.devRewardPCT = cfg.promptValue("DEV_REWARD_PCT", fmt.Sprintf("%.2f", devRewardPCT), false, "float").(float64)
		} else {
			fmt.Printf("Notice: Environment DEV_REWARD_PCT is set to %s\n", os.Getenv("DEV_REWARD_PCT"))
			cfg.devRewardPCT = cfg.getFloatEnv("DEV_REWARD_PCT", devRewardPCT)
		}
	}

	if os.Getenv("DEV_ADDRESS") != "" {
		if os.Getenv("DEV_ADDRESS") == devAddress {
			//fmt.Printf("Warning: Environment DEV_ADDRESS is set to the default value of %s and will be ignored\n", devAddress)
			cfg.devAddress = cfg.promptValue("DEV_ADDRESS", devAddress, true, "string").(string)
		} else {
			fmt.Printf("Notice: Environment DEV_ADDRESS is set to %s\n", os.Getenv("DEV_ADDRESS"))
			cfg.devAddress = os.Getenv("DEV_ADDRESS")
		}
	}

	if os.Getenv("API_HOSTNAME") != "" {
		if os.Getenv("API_HOSTNAME") == apiHostname {
			//fmt.Printf("Warning: Environment API_HOSTNAME is set to the default value of %s and will be ignored\n", apiHostname)
			cfg.apiHostname = cfg.promptValue("API_HOSTNAME", apiHostname, false, "string").(string)
		} else {
			fmt.Printf("Notice: Environment API_HOSTNAME is set to %s\n", os.Getenv("API_HOSTNAME"))
			cfg.apiHostname = os.Getenv("API_HOSTNAME")
		}
	}

	if os.Getenv("ENABLE_API") != "" {
		if os.Getenv("ENABLE_API") == strconv.FormatBool(EnableAPI) {
			//fmt.Printf("Warning: Environment ENABLE_API is set to the default value of %t and will be ignored\n", EnableAPI)
			cfg.enableAPI = cfg.promptValue("ENABLE_API", strconv.FormatBool(EnableAPI), false, "bool").(bool)
		} else {
			fmt.Printf("Notice: Environment ENABLE_API is set to %s\n", os.Getenv("ENABLE_API"))
			cfg.enableAPI = cfg.getBoolEnv("ENABLE_API", EnableAPI)
		}
	}

	if os.Getenv("FUND_WALLET_AMOUNT") != "" {
		if os.Getenv("FUND_WALLET_AMOUNT") == fmt.Sprintf("%.2f", fundWalletAmount) {
			//fmt.Printf("Warning: Environment FUND_WALLET_AMOUNT is set to the default value of %.2f and will be ignored\n", fundWalletAmount)
			cfg.fundWalletAmount = cfg.promptValue("FUND_WALLET_AMOUNT", fmt.Sprintf("%.2f", fundWalletAmount), false, "float").(float64)
		} else {
			fmt.Printf("Notice: Environment FUND_WALLET_AMOUNT is set to %s\n", os.Getenv("FUND_WALLET_AMOUNT"))
			cfg.fundWalletAmount = cfg.getFloatEnv("FUND_WALLET_AMOUNT", fundWalletAmount)
		}
	}

	// step 5: save / update .env file if changes were made
	cfg.save()

	// step 6: display config values that will be used
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

func (c *Config) promptValue(key, defaultValue string, required bool, returnType string) interface{} {
	value := os.Getenv(key)

	if value == "" {
		value = defaultValue
	}

	if required {
		fmt.Printf("Enter value for %s (required): ", key)
	} else {
		fmt.Printf("Enter value for %s (<ENTER> default: %s): ", key, defaultValue)
	}

	fmt.Scanln(&value)

	if required && (value == "" || value == defaultValue) {
		fmt.Println("This is a required value and must be set")
		os.Exit(1)
	}

	if value != defaultValue {
		c.promptUpdate = true
	}

	switch strings.ToLower(returnType) {
	case "string":
		return value
	case "int":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			fmt.Println("Invalid value. Please enter a valid integer.")
			os.Exit(1)
		}
		return intValue
	case "float":
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			fmt.Println("Invalid value. Please enter a valid float.")
			os.Exit(1)
		}
		return floatValue
	case "bool":
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Println("Invalid value. Please enter true or false.")
			os.Exit(1)
		}
		return boolValue
	}

	return value
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

func (c *Config) writeEnvValue(f *os.File, key, value string) {
	_, err := fmt.Fprintf(f, "%s=%s\n", key, value)
	if err != nil {
		log.Fatal("Error writing to .env file")
	}
}

func (c *Config) save() error {
	if c.promptUpdate {
		f, err := os.Create(cfgFile)
		if err != nil {
			return fmt.Errorf("error creating .env file: %s", err)
		}
		defer f.Close()

		c.writeEnvValue(f, "BLOCKCHAIN_NAME", c.blockchainName)
		c.writeEnvValue(f, "BLOCKCHAIN_SYMBOL", c.blockchainSymbol)
		c.writeEnvValue(f, "BLOCK_TIME", fmt.Sprintf("%d", c.blockTime))
		c.writeEnvValue(f, "DIFFICULTY", fmt.Sprintf("%d", c.difficulty))
		c.writeEnvValue(f, "TRANSACTION_FEE", fmt.Sprintf("%.2f", c.transactionFee))
		c.writeEnvValue(f, "MINER_REWARD_PCT", fmt.Sprintf("%.2f", c.minerRewardPCT))
		c.writeEnvValue(f, "MINER_ADDRESS", c.minerAddress)
		c.writeEnvValue(f, "DEV_REWARD_PCT", fmt.Sprintf("%.2f", c.devRewardPCT))
		c.writeEnvValue(f, "DEV_ADDRESS", c.devAddress)
		c.writeEnvValue(f, "API_HOSTNAME", c.apiHostname)
		c.writeEnvValue(f, "ENABLE_API", fmt.Sprintf("%v", c.enableAPI))
		c.writeEnvValue(f, "FUND_WALLET_AMOUNT", fmt.Sprintf("%.2f", c.fundWalletAmount))

		fmt.Println("Updated values have been saved to .env file.")
	} else {
		fmt.Println("No values were modified.")
	}

	return nil
}
