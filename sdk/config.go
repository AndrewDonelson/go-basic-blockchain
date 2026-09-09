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
	Version           string   // New field: Configuration version
	MaxBlockSize      int      // New field: Maximum block size in bytes
	MinTransactionFee float64  // New field: Minimum transaction fee
	IsSeed            bool     // New field: Is this a seed node
	SeedAddress       string   // New field: Address of the seed node to connect to
	Verbose           bool     // Enable verbose logging
	AllowedPeers      []string // If non-empty, only these node IDs may connect
	DifficultyWindow  int      // Blocks between difficulty retargets
	MaxMempoolTxs     int      // Maximum transactions held in the mempool
	promptUpdate      bool
	testing           bool
}

// NewConfig creates a new configuration object with default values.
func NewConfig() *Config {
	home, err := os.UserHomeDir()
	dataPath := "./data"
	if err == nil {
		dataPath = filepath.Join(home, "gbb-data")
	}
	verbose := true // Enable verbose by default
	if v := os.Getenv("VERBOSE"); v == "0" || v == "false" || v == "FALSE" {
		verbose = false
	}
	cfg := &Config{
		DataPath: dataPath,
		Version:  "1.0", // Set initial version
		Verbose:  verbose,
	}

	cfg.setDefaultValues() // Set default values (lowest priority)

	//if !cfg.testing && !fileExists(cfgFile) {
	// if !fileExists(cfgFile) {
	// 	cfg.promptForValues()
	// 	cfg.save()
	// }

	// Load values from environment variables (higher priority)
	cfg.loadFromEnv()

	// Apply command line flags (highest priority)
	cfg.applyCommandLineFlags()

	// Surface a bad configuration at startup rather than at first use. NewConfig
	// never validated what it produced.
	if err := cfg.Validate(); err != nil {
		LogInfof("Configuration is invalid: %v", err)
	}

	return cfg
}

// setDefaultValues sets the default values for the configuration.
func (c *Config) setDefaultValues() {
	if verbose {
		log.Println("Setting default values")
	}

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
	c.DifficultyWindow = defaultDifficultyWindow
	c.MaxMempoolTxs = defaultMaxMempoolTxs
}

// loadFromEnv loads configuration values from environment variables.
// loadFromEnv loads configuration values from environment variables.
func (c *Config) loadFromEnv() {
	c.testing = (os.Getenv("TESTING") == "true")

	if !c.testing {
		// Check for custom environment file path
		envFile := os.Getenv("ENV_FILE")

		// If no custom path is set, use default paths
		if envFile == "" {
			// Try multiple potential paths
			defaultPaths := []string{
				".local.env",                 // Current directory
				"../../.local.env",           // Relative to typical project structure
				"../.local.env",              // One directory up
				"./bin/.local.env",           // In the bin directory
				"/etc/blockchain/.local.env", // System-wide configuration
			}

			for _, path := range defaultPaths {
				if fileExists(path) {
					envFile = path
					break
				}
			}
		}

		// If an environment file is found, load it.
		//
		// A parse failure falls back to defaults. It used to call
		// promptForValues(), i.e. block on an interactive stdin read from inside
		// library initialization -- which hangs a daemon, a container or a CI run
		// forever, or silently consumes input meant for something else.
		if envFile != "" {
			if err := godotenv.Load(envFile); err != nil {
				LogInfof("Error loading environment file [%s]: %v (continuing with defaults)", envFile, err)
			} else {
				LogVerbosef("Loaded environment file: %s", envFile)
			}
		}

		// Proceed with loading environment variables
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
		c.Verbose = getEnvAsBool("VERBOSE", c.Verbose)
		c.AllowedPeers = getEnvAsList("P2P_ALLOWED_PEERS", c.AllowedPeers)
		c.DifficultyWindow = getEnvAsInt("DIFFICULTY_WINDOW", c.DifficultyWindow)
		c.MaxMempoolTxs = getEnvAsInt("MAX_MEMPOOL_TXS", c.MaxMempoolTxs)
	}
}

// loadFromFile loads configuration from a file if it exists
// This function is currently unused but kept for potential future use
//
//nolint:unused
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
	for name := range Args.Flags {
		switch name {
		case "seed":
			c.IsSeed = Args.GetBool("seed")
		case "seed-address":
			c.SeedAddress = Args.GetString("seed-address")
		case "verbose":
			c.Verbose = Args.GetBool("verbose")
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
	// Difficulty feeds a 256-bit shift in difficultyTarget; an out-of-range value
	// there produces a nonsensical target.
	if c.Difficulty < 1 || c.Difficulty > 255 {
		return errors.New("difficulty must be between 1 and 255")
	}
	if c.MaxMempoolTxs < 0 {
		return errors.New("max mempool transactions cannot be negative")
	}

	if c.DifficultyWindow < 0 {
		return errors.New("difficulty window cannot be negative")
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
	log.Printf("- Max Mempool Txs: %d\n", c.MaxMempoolTxs)
	log.Printf("- Min Transaction Fee: %.2f\n", c.MinTransactionFee)
	log.Printf("- Is Seed Node: %v\n", c.IsSeed)
	log.Printf("- Seed Address: %s\n", c.SeedAddress)
}

// Path returns the path to the executable file.
func (c *Config) Path() string {
	ex, err := os.Executable()
	if err != nil {
		// Panicking here killed the process because the OS could not report the
		// executable's location -- recoverable, and not worth a crash.
		LogInfof("Could not determine executable path: %v", err)
		return "."
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

		if err := c.writeEnvValue(f, "BLOCKCHAIN_NAME", c.BlockchainName); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "BLOCKCHAIN_SYMBOL", c.BlockchainSymbol); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "BLOCK_TIME", fmt.Sprintf("%d", c.BlockTime)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "DIFFICULTY", fmt.Sprintf("%d", c.Difficulty)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "TRANSACTION_FEE", fmt.Sprintf("%.2f", c.TransactionFee)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "MINER_REWARD_PCT", fmt.Sprintf("%.2f", c.MinerRewardPCT)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "MINER_ADDRESS", c.MinerAddress); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "DEV_REWARD_PCT", fmt.Sprintf("%.2f", c.DevRewardPCT)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "DEV_ADDRESS", c.DevAddress); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "API_HOSTNAME", c.APIHostName); err != nil {
			return err
		}
		c.writeEnvValue(f, "P2P_HOSTNAME", c.P2PHostName)
		if err := c.writeEnvValue(f, "ENABLE_API", fmt.Sprintf("%v", c.EnableAPI)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "FUND_WALLET_AMOUNT", fmt.Sprintf("%.2f", c.FundWalletAmount)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "TOKEN_COUNT", fmt.Sprintf("%d", c.TokenCount)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "TOKEN_PRICE", fmt.Sprintf("%.2f", c.TokenPrice)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "ALLOW_NEW_TOKENS", fmt.Sprintf("%v", c.AllowNewTokens)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "MAX_MEMPOOL_TXS", fmt.Sprintf("%d", c.MaxMempoolTxs)); err != nil {
			return err
		}

		if err := c.writeEnvValue(f, "MAX_BLOCK_SIZE", fmt.Sprintf("%d", c.MaxBlockSize)); err != nil {
			return err
		}
		if err := c.writeEnvValue(f, "MIN_TRANSACTION_FEE", fmt.Sprintf("%.2f", c.MinTransactionFee)); err != nil {
			return err
		}

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

// promptValue reads a configuration value from the terminal.
//
// On invalid input it falls back to the default and reports the problem. It used
// to call os.Exit(1) at five separate points: a library terminating the host
// process because somebody mistyped a number.
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

	_, _ = fmt.Scanln(&value)

	if required && value == "" {
		fmt.Println("This is a required value and must be set")
		return parseConfigValue(defaultValue, defaultValue, returnType)
	}

	if value != defaultValue {
		c.promptUpdate = true
	}

	return parseConfigValue(value, defaultValue, returnType)
}

// parseConfigValue converts a prompted string to the requested type, falling back
// to the default on malformed input.
func parseConfigValue(value, defaultValue, returnType string) interface{} {
	switch strings.ToLower(returnType) {
	case "int":
		intValue, err := strconv.Atoi(value)
		if err != nil {
			fmt.Printf("Invalid integer %q; using %s\n", value, defaultValue)
			intValue, _ = strconv.Atoi(defaultValue)
		}
		return intValue
	case "int64":
		int64Value, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			fmt.Printf("Invalid 64-bit integer %q; using %s\n", value, defaultValue)
			int64Value, _ = strconv.ParseInt(defaultValue, 10, 64)
		}
		return int64Value
	case "float":
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			fmt.Printf("Invalid number %q; using %s\n", value, defaultValue)
			floatValue, _ = strconv.ParseFloat(defaultValue, 64)
		}
		return floatValue
	case "bool":
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("Invalid boolean %q; using %s\n", value, defaultValue)
			boolValue, _ = strconv.ParseBool(defaultValue)
		}
		return boolValue
	default:
		return value
	}
}

func (c *Config) writeEnvValue(f *os.File, key, value string) error {
	if _, err := fmt.Fprintf(f, "%s=%s\n", key, value); err != nil {
		return fmt.Errorf("error writing to .env file: %w", err)
	}
	return nil
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

// getEnvAsList reads a comma-separated environment variable.
func getEnvAsList(key string, fallback []string) []string {
	raw := getEnv(key, "")
	if strings.TrimSpace(raw) == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getEnvAsBool(key string, fallback bool) bool {
	strValue := getEnv(key, "")
	if value, err := strconv.ParseBool(strValue); err == nil {
		return value
	}
	return fallback
}

// fileExists checks if a file exists and is not a directory.
//
// Any Stat error other than not-exist (permission denied, ENOTDIR, a symlink
// loop) leaves info nil, and the old code called info.IsDir() on it regardless --
// a nil dereference during NewConfig(), which probes /etc/blockchain/.local.env
// among other paths.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
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
