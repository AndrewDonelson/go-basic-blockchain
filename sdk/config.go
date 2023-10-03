// Package sdk is a software development kit for building blockchain applications.
// File sdk/config.go - The main Config file
package sdk

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the configuration for the blockchain.
type Config struct {
	BlockchainName   string
	BlockchainSymbol string
	BlockTime        int
	Difficulty       int
	TransactionFee   float64
	MinerRewardPCT   float64
	MinerAddress     string
	DevRewardPCT     float64
	DevAddress       string
	APIHostName      string
	P2PHostName      string
	EnableAPI        bool
	FundWalletAmount float64
	TokenCount       int64   // This is the total number of tokens that will be created intially
	TokenPrice       float64 // This is the set price of each token
	AllowNewTokens   bool    // Set this to true if you want to allow new tokens to be created besides the initial tokens
	promptUpdate     bool    // This is used internally to check if user added/changed default value from prompt
	testing          bool    // This is used internally to check if the code is running in test mode
	DataPath         string  // This is the path where all data is stored
}

// NewConfig returns a new config.
func NewConfig() *Config {
	// step 1: create a new actual config
	cfg := &Config{
		DataPath: filepath.Join(".", "data"),
	}

	// step 2: set the default values from the constants
	cfg.BlockchainName = BlockchainName
	cfg.BlockchainSymbol = BlockchainSymbol
	cfg.BlockTime = blockTimeInSec
	cfg.Difficulty = proofOfWorkDifficulty
	cfg.TransactionFee = transactionFee
	cfg.MinerRewardPCT = minerRewardPCT
	cfg.MinerAddress = minerAddress
	cfg.DevRewardPCT = devRewardPCT
	cfg.DevAddress = devAddress
	cfg.APIHostName = apiHostname
	cfg.P2PHostName = p2pHostname
	cfg.EnableAPI = EnableAPI
	cfg.FundWalletAmount = fundWalletAmount
	cfg.TokenCount = tokenCount
	cfg.TokenPrice = tokenPrice
	cfg.AllowNewTokens = allowNewTokens

	cfg.testing = (flag.Lookup("test.v") != nil)

	if verbose {
		if !cfg.testing {
			//  step 3: Load all values in the .env file if it exists
			err := godotenv.Load(cfgFile)
			if err != nil {
				log.Fatal("Error loading .env file")
			}

			// step 4: set the values from the environment variables
			if os.Getenv("BLOCKCHAIN_NAME") != "" {
				if os.Getenv("BLOCKCHAIN_NAME") == BlockchainName {
					cfg.BlockchainName = cfg.promptValue("BLOCKCHAIN_NAME", BlockchainName, false, "string").(string)
				} else {
					fmt.Printf("Notice: Environment BLOCKCHAIN_NAME is set to %s\n", os.Getenv("BLOCKCHAIN_NAME"))
					cfg.BlockchainName = os.Getenv("BLOCKCHAIN_NAME")
				}
			}

			if os.Getenv("BLOCKCHAIN_SYMBOL") != "" {
				if os.Getenv("BLOCKCHAIN_SYMBOL") == BlockchainSymbol {
					cfg.BlockchainSymbol = cfg.promptValue("BLOCKCHAIN_SYMBOL", BlockchainSymbol, false, "string").(string)
				} else {
					fmt.Printf("Notice: Environment BLOCKCHAIN_SYMBOL is set to %s\n", os.Getenv("BLOCKCHAIN_SYMBOL"))
					cfg.BlockchainSymbol = os.Getenv("BLOCKCHAIN_SYMBOL")
				}
			}

			if os.Getenv("BLOCK_TIME") != "" {
				if os.Getenv("BLOCK_TIME") == strconv.Itoa(blockTimeInSec) {
					cfg.BlockTime = cfg.promptValue("BLOCK_TIME", fmt.Sprintf("%d", blockTimeInSec), false, "int").(int)
				} else {
					fmt.Printf("Notice: Environment BLOCK_TIME is set to %s\n", os.Getenv("BLOCK_TIME"))
					cfg.BlockTime = cfg.getIntEnv("BLOCK_TIME", blockTimeInSec)
				}
			}

			if os.Getenv("DIFFICULTY") != "" {
				if os.Getenv("DIFFICULTY") == strconv.Itoa(proofOfWorkDifficulty) {
					cfg.Difficulty = cfg.promptValue("DIFFICULTY", fmt.Sprintf("%d", proofOfWorkDifficulty), false, "int").(int)
				} else {
					fmt.Printf("Notice: Environment DIFFICULTY is set to %s\n", os.Getenv("DIFFICULTY"))
					cfg.Difficulty = cfg.getIntEnv("DIFFICULTY", proofOfWorkDifficulty)
				}
			}

			if os.Getenv("TRANSACTION_FEE") != "" {
				if os.Getenv("TRANSACTION_FEE") == fmt.Sprintf("%.2f", transactionFee) {
					cfg.TransactionFee = cfg.promptValue("TRANSACTION_FEE", fmt.Sprintf("%.2f", transactionFee), false, "float").(float64)
				} else {
					fmt.Printf("Notice: Environment TRANSACTION_FEE is set to %s\n", os.Getenv("TRANSACTION_FEE"))
					cfg.TransactionFee = cfg.getFloatEnv("TRANSACTION_FEE", transactionFee)
				}
			}

			// this is the node wallet (miner) address
			if os.Getenv("MINER_ADDRESS") != "" {
				if os.Getenv("MINER_ADDRESS") == minerAddress {
					if cfg.PromptYesNo("Do you want have a miner address?") {
						cfg.MinerAddress = cfg.promptValue("MINER_ADDRESS", minerAddress, true, "string").(string)
					} else {
						if cfg.PromptYesNo("Do you want to crate a new wallet for the miner address?") {
							walletName, walletPassPhrase, walletTags := cfg.PromptWalletInfo()
							minerWallet, err := NewWallet(walletName, walletPassPhrase, walletTags)
							if err != nil {
								log.Fatal(err)
							}
							cfg.MinerAddress = minerWallet.GetAddress()
						}
					}

					if cfg.MinerAddress == "" || cfg.MinerAddress == minerAddress {
						log.Fatal("Error: MINER_ADDRESS is required")
					}
				} else {
					fmt.Printf("Notice: Environment MINER_ADDRESS is set to %s\n", os.Getenv("MINER_ADDRESS"))
					cfg.MinerAddress = os.Getenv("MINER_ADDRESS")
				}
			}

			if os.Getenv("MINER_REWARD_PCT") != "" {
				if os.Getenv("MINER_REWARD_PCT") == fmt.Sprintf("%.2f", minerRewardPCT) {
					cfg.MinerRewardPCT = cfg.promptValue("MINER_REWARD_PCT", fmt.Sprintf("%.2f", minerRewardPCT), false, "float").(float64)
				} else {
					fmt.Printf("Notice: Environment MINER_REWARD_PCT is set to %s\n", os.Getenv("MINER_REWARD_PCT"))
					cfg.MinerRewardPCT = cfg.getFloatEnv("MINER_REWARD_PCT", minerRewardPCT)
				}
			}

			// This is the blockchain developer's wallet address (not the node wallet) - it is used to support (reward) the developer || project
			// TODO: move this into the genesis block - not here
			if os.Getenv("DEV_REWARD_PCT") != "" {
				if os.Getenv("DEV_REWARD_PCT") == fmt.Sprintf("%.2f", devRewardPCT) {
					cfg.DevRewardPCT = cfg.promptValue("DEV_REWARD_PCT", fmt.Sprintf("%.2f", devRewardPCT), false, "float").(float64)
				} else {
					fmt.Printf("Notice: Environment DEV_REWARD_PCT is set to %s\n", os.Getenv("DEV_REWARD_PCT"))
					cfg.DevRewardPCT = cfg.getFloatEnv("DEV_REWARD_PCT", devRewardPCT)
				}
			}

			if os.Getenv("DEV_ADDRESS") != "" {
				if os.Getenv("DEV_ADDRESS") == devAddress {
					cfg.DevAddress = cfg.promptValue("DEV_ADDRESS", devAddress, true, "string").(string)
				} else {
					fmt.Printf("Notice: Environment DEV_ADDRESS is set to %s\n", os.Getenv("DEV_ADDRESS"))
					cfg.DevAddress = os.Getenv("DEV_ADDRESS")
				}
			}

			if os.Getenv("P2P_HOSTNAME") != "" {
				if os.Getenv("P2P_HOSTNAME") == p2pHostname {
					cfg.P2PHostName = cfg.promptValue("P2P_HOSTNAME", p2pHostname, false, "string").(string)
				} else {
					fmt.Printf("Notice: Environment P2P_HOSTNAME is set to %s\n", os.Getenv("P2P_HOSTNAME"))
					cfg.P2PHostName = os.Getenv("P2P_HOSTNAME")
				}
			}

			if os.Getenv("API_HOSTNAME") != "" {
				if os.Getenv("API_HOSTNAME") == apiHostname {
					cfg.APIHostName = cfg.promptValue("API_HOSTNAME", apiHostname, false, "string").(string)
				} else {
					fmt.Printf("Notice: Environment API_HOSTNAME is set to %s\n", os.Getenv("API_HOSTNAME"))
					cfg.APIHostName = os.Getenv("API_HOSTNAME")
				}
			}

			if os.Getenv("ENABLE_API") != "" {
				if os.Getenv("ENABLE_API") == strconv.FormatBool(EnableAPI) {
					cfg.EnableAPI = cfg.promptValue("ENABLE_API", strconv.FormatBool(EnableAPI), false, "bool").(bool)
				} else {
					fmt.Printf("Notice: Environment ENABLE_API is set to %s\n", os.Getenv("ENABLE_API"))
					cfg.EnableAPI = cfg.getBoolEnv("ENABLE_API", EnableAPI)
				}
			}

			if os.Getenv("FUND_WALLET_AMOUNT") != "" {
				if os.Getenv("FUND_WALLET_AMOUNT") == fmt.Sprintf("%.2f", fundWalletAmount) {
					cfg.FundWalletAmount = cfg.promptValue("FUND_WALLET_AMOUNT", fmt.Sprintf("%.2f", fundWalletAmount), false, "float").(float64)
				} else {
					fmt.Printf("Notice: Environment FUND_WALLET_AMOUNT is set to %s\n", os.Getenv("FUND_WALLET_AMOUNT"))
					cfg.FundWalletAmount = cfg.getFloatEnv("FUND_WALLET_AMOUNT", fundWalletAmount)
				}
			}

			if os.Getenv("TOKEN_COUNT") != "" {
				tCount, err := strconv.ParseInt(os.Getenv("TOKEN_COUNT"), 10, 64)
				if err != nil {
					log.Fatal(err)
				}
				if tCount == tokenCount {
					newCount := cfg.promptValue("TOKEN_COUNT", fmt.Sprintf("%d", tokenCount), false, "int").(int64)
					cfg.TokenCount = newCount
				} else {
					fmt.Printf("Notice: Environment TOKEN_COUNT is set to %d\n", tCount)
					cfg.TokenCount = tCount
				}
			}

			if os.Getenv("TOKEN_PRICE") != "" {
				if os.Getenv("TOKEN_PRICE") == fmt.Sprintf("%.2f", tokenPrice) {
					cfg.TokenPrice = cfg.promptValue("TOKEN_PRICE", fmt.Sprintf("%.2f", tokenPrice), false, "float").(float64)
				} else {
					fmt.Printf("Notice: Environment TOKEN_PRICE is set to %s\n", os.Getenv("TOKEN_PRICE"))
					cfg.TokenPrice = cfg.getFloatEnv("TOKEN_PRICE", tokenPrice)
				}
			}

			if os.Getenv("ALLOW_NEW_TOKENS") != "" {
				if os.Getenv("ALLOW_NEW_TOKENS") == strconv.FormatBool(allowNewTokens) {
					cfg.AllowNewTokens = cfg.promptValue("ALLOW_NEW_TOKENS", strconv.FormatBool(allowNewTokens), false, "bool").(bool)
				} else {
					fmt.Printf("Notice: Environment ALLOW_NEW_TOKENS is set to %s\n", os.Getenv("ALLOW_NEW_TOKENS"))
					cfg.AllowNewTokens = cfg.getBoolEnv("ALLOW_NEW_TOKENS", allowNewTokens)
				}
			}

			if os.Getenv("DATA_PATH") != "" {
				if os.Getenv("DATA_PATH") == cfg.DataPath {
					cfg.DevAddress = cfg.promptValue("DATA_PATH", cfg.DataPath, false, "string").(string)
				} else {
					fmt.Printf("Notice: Environment DATA_PATH is set to %s\n", os.Getenv("DATA_PATH"))
					cfg.DataPath = os.Getenv("DATA_PATH")
				}
			}

			// step 5: save / update .env file if changes were made
			cfg.save()
		}
	}

	// step 6: display config values that will be used

	return cfg

}

// Show displays the configuration values that will be used.
func (c *Config) Show() {
	fmt.Println("Using these Configuration Values:")
	fmt.Printf("- Blockchain Name: %s\n", c.BlockchainName)
	fmt.Printf("- Blockchain Symbol: %s\n", c.BlockchainSymbol)
	fmt.Printf("- Block Time: %d seconds\n", c.BlockTime)
	fmt.Printf("- Difficulty: %d\n", c.Difficulty)
	fmt.Printf("- Transaction Fee: %.2f\n", c.TransactionFee)
	fmt.Printf("- Miner Reward Percentage: %.2f%%\n", c.MinerRewardPCT)
	fmt.Printf("- Miner Address: %s\n", c.MinerAddress)
	fmt.Printf("- Developer Reward Percentage: %.2f%%\n", c.DevRewardPCT)
	fmt.Printf("- Developer Address: %s\n", c.DevAddress)
	fmt.Printf("- API Hostname: %s\n", c.APIHostName)
	fmt.Printf("- Enable API: %v\n", c.EnableAPI)
	fmt.Printf("- Fund Wallet Amount: %.2f\n", c.FundWalletAmount)
	fmt.Printf("- Token Count: %d\n", c.TokenCount)
	fmt.Printf("- Token Price: %.2f\n", c.TokenPrice)
	fmt.Printf("- Allow New Tokens: %v\n", c.AllowNewTokens)
	fmt.Printf("- Data Path: %s\n", c.DataPath)
}

// Path returns the path to the executable file
func (c *Config) Path() string {

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}

	return filepath.Dir(ex)
}

// promptValue prompts the user for a value and returns the value
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

// PromptYesNo prompts the user with a given question and returns a bool value based on their response.
func (c *Config) PromptYesNo(question string) bool {
	affirmativeResponses := []string{"yes", "y", "true", "t"}
	negativeResponses := []string{"no", "n", "false", "f"}

	for {
		response := strings.ToLower(c.promptValue(question, "", true, "string").(string))
		for _, affirmative := range affirmativeResponses {
			if response == affirmative {
				return true
			}
		}
		for _, negative := range negativeResponses {
			if response == negative {
				return false
			}
		}
		fmt.Println("Invalid response. Please enter a valid yes/no value.")
	}
}

// PromptWalletInfo prompts the user to enter wallet information such as Name, passphrase, and comma-delimited list of tags.
func (c *Config) PromptWalletInfo() (walletName string, walletPass string, walletTags []string) {
	walletName = c.promptValue("Wallet Name", "", false, "string").(string)
	walletPass = c.promptValue("Passphrase", "", true, "string").(string)
	walletTags = c.promptTags()

	return
}

// promptTags prompts the user to enter a comma-delimited list of tags for the wallet.
func (c *Config) promptTags() []string {
	tagsStr := c.promptValue("Tags (comma-separated)", "", false, "string").(string)
	tags := strings.Split(tagsStr, ",")
	for i := 0; i < len(tags); i++ {
		tags[i] = strings.TrimSpace(tags[i])
	}
	return tags
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

// getFloatEnv returns the value of an environment variable as a float64. If the environment variable is not set or
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

// getBoolEnv returns the value of an environment variable as a bool. If the environment variable is not set or
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

// writeEnvValue writes a key/value pair to the .env file
func (c *Config) writeEnvValue(f *os.File, key, value string) {
	_, err := fmt.Fprintf(f, "%s=%s\n", key, value)
	if err != nil {
		log.Fatal("Error writing to .env file")
	}
}

// save writes the current configuration to the .env file
func (c *Config) save() error {
	if c.promptUpdate {
		f, err := os.Create(cfgFile)
		if err != nil {
			return fmt.Errorf("error creating .env file: %s", err)
		}
		defer f.Close()

		c.writeEnvValue(f, "BLOCKCHAIN_NAME", c.BlockchainName)
		c.writeEnvValue(f, "BLOCKCHAIN_SYMBOL", c.BlockchainSymbol)
		c.writeEnvValue(f, "BLOCK_TIME", fmt.Sprintf("%d", c.BlockTime))
		c.writeEnvValue(f, "DIFFICULTY", fmt.Sprintf("%d", c.Difficulty))
		c.writeEnvValue(f, "TRANSACTION_FEE", fmt.Sprintf("%.2f", c.TransactionFee))
		c.writeEnvValue(f, "MINER_REWARD_PCT", fmt.Sprintf("%.2f", c.MinerRewardPCT))
		c.writeEnvValue(f, "MINER_ADDRESS", c.MinerAddress)
		c.writeEnvValue(f, "DEV_REWARD_PCT", fmt.Sprintf("%.2f", c.DevRewardPCT))
		c.writeEnvValue(f, "DEV_ADDRESS", c.DevAddress)
		c.writeEnvValue(f, "API_HOSTNAME", c.APIHostName)
		c.writeEnvValue(f, "ENABLE_API", fmt.Sprintf("%v", c.EnableAPI))
		c.writeEnvValue(f, "FUND_WALLET_AMOUNT", fmt.Sprintf("%.2f", c.FundWalletAmount))

		fmt.Println("Updated values have been saved to .env file.")
	} else {
		fmt.Println("No values were modified.")
	}

	return nil
}
