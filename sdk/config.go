// Package sdk is a software development kit for building blockchain applications.
// File sdk/config.go - The main Config file

package sdk

import (
	"encoding/json"
	"errors"
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
	BlockchainName    string
	BlockchainSymbol  string
	BlockTime         int
	Difficulty        int
	TransactionFee    float64
	MinerRewardPCT    float64
	MinerAddress      string
	DevRewardPCT      float64
	DevAddress        string
	APIHostName       string
	P2PHostName       string
	EnableAPI         bool
	FundWalletAmount  float64
	TokenCount        int64
	TokenPrice        float64
	AllowNewTokens    bool
	DataPath          string
	GMailEmail        string
	GMailPassword     string
	Domain            string
	Version           string  // New field: Configuration version
	MaxBlockSize      int     // New field: Maximum block size in bytes
	MinTransactionFee float64 // New field: Minimum transaction fee
	IsSeed            bool    // New field: Is this a seed node
	SeedAddress       string  // New field: Address of the seed node to connect to
	promptUpdate      bool
	testing           bool
}

// NewConfig creates a new configuration object with default values.
func NewConfig() *Config {
	cfg := &Config{
		DataPath: filepath.Join(".", "data"),
		Version:  "1.0", // Set initial version
	}

	cfg.setDefaultValues()
	cfg.loadFromEnv()
	cfg.loadFromFile()
	cfg.applyCommandLineFlags()

	if !cfg.testing && !fileExists(cfgFile) {
		cfg.promptForValues()
		cfg.save()
	}

	return cfg
}

// setDefaultValues sets the default values for the configuration.
func (c *Config) setDefaultValues() {
	c.BlockchainName = BlockchainName
	c.BlockchainSymbol = BlockchainSymbol
	c.BlockTime = blockTimeInSec
	c.Difficulty = proofOfWorkDifficulty
	c.TransactionFee = transactionFee
	c.MinerRewardPCT = minerRewardPCT
	c.MinerAddress = minerAddress
	c.DevRewardPCT = devRewardPCT
	c.DevAddress = devAddress
	c.APIHostName = apiHostname
	c.P2PHostName = p2pHostname
	c.EnableAPI = EnableAPI
	c.FundWalletAmount = fundWalletAmount
	c.TokenCount = tokenCount
	c.TokenPrice = tokenPrice
	c.AllowNewTokens = allowNewTokens
	c.MaxBlockSize = MaxBlockSize
	c.MinTransactionFee = minTransactionFee
}

// loadFromEnv loads configuration values from environment variables.
func (c *Config) loadFromEnv() {
	c.testing = (os.Getenv("TESTING") == "true")

	if !c.testing {
		envFile := os.Getenv("ENV_FILE")
		if envFile == "" {
			envFile = cfgFile
		}

		err := godotenv.Load(envFile)
		if err != nil {
			log.Printf("Error loading [%s] environment file: %v", envFile, err)
		} else {
			log.Printf("Loaded [%s] environment file", envFile)
		}

		c.BlockchainName = getEnv("BLOCKCHAIN_NAME", c.BlockchainName)
		c.BlockchainSymbol = getEnv("BLOCKCHAIN_SYMBOL", c.BlockchainSymbol)
		c.BlockTime = getEnvAsInt("BLOCK_TIME", c.BlockTime)
		c.Difficulty = getEnvAsInt("DIFFICULTY", c.Difficulty)
		c.TransactionFee = getEnvAsFloat("TRANSACTION_FEE", c.TransactionFee)
		c.MinerRewardPCT = getEnvAsFloat("MINER_REWARD_PCT", c.MinerRewardPCT)
		c.MinerAddress = getEnv("MINER_ADDRESS", c.MinerAddress)
		c.DevRewardPCT = getEnvAsFloat("DEV_REWARD_PCT", c.DevRewardPCT)
		c.DevAddress = getEnv("DEV_ADDRESS", c.DevAddress)
		c.APIHostName = getEnv("API_HOSTNAME", c.APIHostName)
		c.P2PHostName = getEnv("P2P_HOSTNAME", c.P2PHostName)
		c.EnableAPI = getEnvAsBool("ENABLE_API", c.EnableAPI)
		c.FundWalletAmount = getEnvAsFloat("FUND_WALLET_AMOUNT", c.FundWalletAmount)
		c.TokenCount = getEnvAsInt64("TOKEN_COUNT", c.TokenCount)
		c.TokenPrice = getEnvAsFloat("TOKEN_PRICE", c.TokenPrice)
		c.AllowNewTokens = getEnvAsBool("ALLOW_NEW_TOKENS", c.AllowNewTokens)
		c.DataPath = getEnv("DATA_PATH", c.DataPath)
		c.GMailEmail = getEnv("GMAIL_EMAIL", c.GMailEmail)
		c.GMailPassword = getEnv("GMAIL_PASSWORD", c.GMailPassword)
		c.Domain = getEnv("DOMAIN", c.Domain)
		c.MaxBlockSize = getEnvAsInt("MAX_BLOCK_SIZE", c.MaxBlockSize)
		c.MinTransactionFee = getEnvAsFloat("MIN_TRANSACTION_FEE", c.MinTransactionFee)
	}
}

// loadFromFile loads configuration from a file if it exists
func (c *Config) loadFromFile() {
	if fileExists(cfgFile) {
		data, err := os.ReadFile(cfgFile)
		if err != nil {
			log.Printf("Error reading config file: %v", err)
			return
		}
		err = json.Unmarshal(data, c)
		if err != nil {
			log.Printf("Error parsing config file: %v", err)
			return
		}
		log.Printf("Loaded configuration from file: %s", cfgFile)
	}
}

// applyCommandLineFlags applies command line flags to override config values
func (c *Config) applyCommandLineFlags() {
	for name, _ := range Args.Flags {
		switch name {
		case "seed":
			c.IsSeed = Args.GetBool("seed")
		case "seed-address":
			c.SeedAddress = Args.GetString("seed-address")
			// Add more cases for other flags as needed
		}
	}
}

// promptForValues prompts the user for configuration values.
func (c *Config) promptForValues() {
	c.BlockchainName = c.promptString("BLOCKCHAIN_NAME", c.BlockchainName)
	c.BlockchainSymbol = c.promptString("BLOCKCHAIN_SYMBOL", c.BlockchainSymbol)
	c.BlockTime = c.promptInt("BLOCK_TIME", c.BlockTime)
	c.Difficulty = c.promptInt("DIFFICULTY", c.Difficulty)
	c.TransactionFee = c.promptFloat("TRANSACTION_FEE", c.TransactionFee)
	c.MinerRewardPCT = c.promptFloat("MINER_REWARD_PCT", c.MinerRewardPCT)
	c.MinerAddress = c.promptString("MINER_ADDRESS", c.MinerAddress)
	c.DevRewardPCT = c.promptFloat("DEV_REWARD_PCT", c.DevRewardPCT)
	c.DevAddress = c.promptString("DEV_ADDRESS", c.DevAddress)
	c.APIHostName = c.promptString("API_HOSTNAME", c.APIHostName)
	c.P2PHostName = c.promptString("P2P_HOSTNAME", c.P2PHostName)
	c.EnableAPI = c.promptBool("ENABLE_API", c.EnableAPI)
	c.FundWalletAmount = c.promptFloat("FUND_WALLET_AMOUNT", c.FundWalletAmount)
	c.TokenCount = c.promptInt64("TOKEN_COUNT", c.TokenCount)
	c.TokenPrice = c.promptFloat("TOKEN_PRICE", c.TokenPrice)
	c.AllowNewTokens = c.promptBool("ALLOW_NEW_TOKENS", c.AllowNewTokens)
	c.MaxBlockSize = c.promptInt("MAX_BLOCK_SIZE", c.MaxBlockSize)
	c.MinTransactionFee = c.promptFloat("MIN_TRANSACTION_FEE", c.MinTransactionFee)
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.BlockchainName == "" {
		return errors.New("blockchain name cannot be empty")
	}
	if c.BlockchainSymbol == "" {
		return errors.New("blockchain symbol cannot be empty")
	}
	if c.BlockTime <= 0 {
		return errors.New("block time must be positive")
	}
	if c.Difficulty < 0 {
		return errors.New("difficulty cannot be negative")
	}
	if c.TransactionFee < 0 {
		return errors.New("transaction fee cannot be negative")
	}
	if c.MinerRewardPCT < 0 || c.MinerRewardPCT > 100 {
		return errors.New("miner reward percentage must be between 0 and 100")
	}
	if c.DevRewardPCT < 0 || c.DevRewardPCT > 100 {
		return errors.New("developer reward percentage must be between 0 and 100")
	}
	if c.FundWalletAmount < 0 {
		return errors.New("fund wallet amount cannot be negative")
	}
	if c.TokenCount < 0 {
		return errors.New("token count cannot be negative")
	}
	if c.TokenPrice < 0 {
		return errors.New("token price cannot be negative")
	}
	if c.MaxBlockSize <= 0 {
		return errors.New("max block size must be positive")
	}
	if c.MinTransactionFee < 0 {
		return errors.New("minimum transaction fee cannot be negative")
	}
	return nil
}

// Show displays the configuration values.
func (c *Config) Show() {
	log.Println("Current Configuration:")
	log.Printf("- Blockchain Name: %s\n", c.BlockchainName)
	log.Printf("- Blockchain Symbol: %s\n", c.BlockchainSymbol)
	log.Printf("- Block Time: %d seconds\n", c.BlockTime)
	log.Printf("- Difficulty: %d\n", c.Difficulty)
	log.Printf("- Transaction Fee: %.2f\n", c.TransactionFee)
	log.Printf("- Miner Reward Percentage: %.2f%%\n", c.MinerRewardPCT)
	log.Printf("- Miner Address: %s\n", c.MinerAddress)
	log.Printf("- Developer Reward Percentage: %.2f%%\n", c.DevRewardPCT)
	log.Printf("- Developer Address: %s\n", c.DevAddress)
	log.Printf("- API Hostname: %s\n", c.APIHostName)
	log.Printf("- P2P Hostname: %s\n", c.P2PHostName)
	log.Printf("- Enable API: %v\n", c.EnableAPI)
	log.Printf("- Fund Wallet Amount: %.2f\n", c.FundWalletAmount)
	log.Printf("- Token Count: %d\n", c.TokenCount)
	log.Printf("- Token Price: %.2f\n", c.TokenPrice)
	log.Printf("- Allow New Tokens: %v\n", c.AllowNewTokens)
	log.Printf("- Data Path: %s\n", c.DataPath)
	log.Printf("- Max Block Size: %d bytes\n", c.MaxBlockSize)
	log.Printf("- Min Transaction Fee: %.2f\n", c.MinTransactionFee)
	log.Printf("- Is Seed Node: %v\n", c.IsSeed)
	log.Printf("- Seed Address: %s\n", c.SeedAddress)
}

// Path returns the path to the executable file.
func (c *Config) Path() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

// save writes the current configuration to the .env file.
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
		c.writeEnvValue(f, "P2P_HOSTNAME", c.P2PHostName)
		c.writeEnvValue(f, "ENABLE_API", fmt.Sprintf("%v", c.EnableAPI))
		c.writeEnvValue(f, "FUND_WALLET_AMOUNT", fmt.Sprintf("%.2f", c.FundWalletAmount))
		c.writeEnvValue(f, "TOKEN_COUNT", fmt.Sprintf("%d", c.TokenCount))
		c.writeEnvValue(f, "TOKEN_PRICE", fmt.Sprintf("%.2f", c.TokenPrice))
		c.writeEnvValue(f, "ALLOW_NEW_TOKENS", fmt.Sprintf("%v", c.AllowNewTokens))
		c.writeEnvValue(f, "MAX_BLOCK_SIZE", fmt.Sprintf("%d", c.MaxBlockSize))
		c.writeEnvValue(f, "MIN_TRANSACTION_FEE", fmt.Sprintf("%.2f", c.MinTransactionFee))

		log.Println("Updated values have been saved to .env file.")
	} else {
		log.Println("No values were modified.")
	}

	return nil
}

// Helper functions

func (c *Config) promptString(key, defaultValue string) string {
	value := c.promptValue(key, defaultValue, false, "string").(string)
	if value != defaultValue {
		c.promptUpdate = true
	}
	return value
}

func (c *Config) promptInt(key string, defaultValue int) int {
	value := c.promptValue(key, fmt.Sprintf("%d", defaultValue), false, "int").(int)
	if value != defaultValue {
		c.promptUpdate = true
	}
	return value
}

func (c *Config) promptInt64(key string, defaultValue int64) int64 {
	value := c.promptValue(key, fmt.Sprintf("%d", defaultValue), false, "int64").(int64)
	if value != defaultValue {
		c.promptUpdate = true
	}
	return value
}

func (c *Config) promptFloat(key string, defaultValue float64) float64 {
	value := c.promptValue(key, fmt.Sprintf("%.2f", defaultValue), false, "float").(float64)
	if value != defaultValue {
		c.promptUpdate = true
	}
	return value
}

func (c *Config) promptBool(key string, defaultValue bool) bool {
	value := c.promptValue(key, fmt.Sprintf("%v", defaultValue), false, "bool").(bool)
	if value != defaultValue {
		c.promptUpdate = true
	}
	return value
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
	case "int64":
		int64Value, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			fmt.Println("Invalid value. Please enter a valid 64-bit integer.")
			os.Exit(1)
		}
		return int64Value
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

func (c *Config) writeEnvValue(f *os.File, key, value string) {
	_, err := fmt.Fprintf(f, "%s=%s\n", key, value)
	if err != nil {
		log.Fatal("Error writing to .env file")
	}
}

// Helper functions for environment variable handling

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if value, err := strconv.Atoi(strValue); err == nil {
		return value
	}
	return fallback
}

func getEnvAsInt64(key string, fallback int64) int64 {
	strValue := getEnv(key, "")
	if value, err := strconv.ParseInt(strValue, 10, 64); err == nil {
		return value
	}
	return fallback
}

func getEnvAsFloat(key string, fallback float64) float64 {
	strValue := getEnv(key, "")
	if value, err := strconv.ParseFloat(strValue, 64); err == nil {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	strValue := getEnv(key, "")
	if value, err := strconv.ParseBool(strValue); err == nil {
		return value
	}
	return fallback
}

// fileExists checks if a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
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
		log.Println("Invalid response. Please enter a valid yes/no value.")
	}
}

// PromptWalletInfo prompts the user to enter wallet information.
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
