// Package sdk is a software development kit for building blockchain applications.
// File sdk/config.go - The main Config file

package sdk

import (
	"encoding/hex"
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

	// Advisory only, and verbose: NewConfig cannot return an error, so this is a
	// hint for anyone building a Config directly. The authority is newNode, which
	// refuses to start on an invalid configuration and reports it once, with its
	// line breaks intact -- logging it here as well produced the same list twice,
	// the first copy flattened onto one line by the log sanitiser.
	if err := cfg.Validate(); err != nil {
		LogVerbosef("Configuration is invalid: %v", err)
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

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	var problems []string

	if c.BlockchainName == "" {
		problems = append(problems, "blockchain name cannot be empty")
	}
	if c.BlockchainSymbol == "" {
		problems = append(problems, "blockchain symbol cannot be empty")
	}
	if c.BlockTime <= 0 {
		problems = append(problems, "block time must be positive")
	}
	// Difficulty feeds a 256-bit shift in difficultyTarget; an out-of-range value
	// there produces a nonsensical target.
	if c.Difficulty < 1 || c.Difficulty > 255 {
		problems = append(problems, "difficulty must be between 1 and 255")
	}
	if c.MaxMempoolTxs < 0 {
		problems = append(problems, "max mempool transactions cannot be negative")
	}

	if c.DifficultyWindow < 0 {
		problems = append(problems, "difficulty window cannot be negative")
	}
	if c.TransactionFee < 0 {
		problems = append(problems, "transaction fee cannot be negative")
	}
	if c.MinerRewardPCT < 0 || c.MinerRewardPCT > 100 {
		problems = append(problems, "miner reward percentage must be between 0 and 100")
	}
	if c.DevRewardPCT < 0 || c.DevRewardPCT > 100 {
		problems = append(problems, "developer reward percentage must be between 0 and 100")
	}
	if c.FundWalletAmount < 0 {
		problems = append(problems, "fund wallet amount cannot be negative")
	}
	if c.TokenCount < 0 {
		problems = append(problems, "token count cannot be negative")
	}
	if c.TokenPrice < 0 {
		problems = append(problems, "token price cannot be negative")
	}
	if c.MaxBlockSize <= 0 {
		problems = append(problems, "max block size must be positive")
	}
	if c.MinTransactionFee < 0 {
		problems = append(problems, "minimum transaction fee cannot be negative")
	}

	// Credentials are checked here rather than at the point of use.
	//
	// A bad API key used to surface only when the middleware was built, as
	// "failed to create API" with the real reason on a separate line; a weak node
	// wallet passphrase surfaced later still, from wallet creation. Each restart
	// revealed exactly one problem, so a misconfigured .env took as many attempts
	// as it had mistakes. Everything is reported together now.
	if key := getEnv(envBlockchainAPIKey, ""); key != "" {
		if _, err := hex.DecodeString(key); err != nil {
			problems = append(problems,
				fmt.Sprintf("%s must be hexadecimal (try: openssl rand -hex 32): %v",
					envBlockchainAPIKey, err))
		}
	}
	if seed := getEnv(envServerSeed, ""); seed != "" {
		if _, err := hex.DecodeString(seed); err != nil {
			problems = append(problems,
				fmt.Sprintf("%s must be hexadecimal (try: openssl rand -hex 32): %v",
					envServerSeed, err))
		}
	}
	if pass := getEnv(envNodeWalletPassphrase, ""); pass != "" {
		if err := testPasswordStrength(pass); err != nil {
			problems = append(problems,
				fmt.Sprintf("%s is too weak: %v", envNodeWalletPassphrase, err))
		}
	}

	if len(problems) == 1 {
		return errors.New(problems[0])
	}
	if len(problems) > 1 {
		return fmt.Errorf("%d configuration problems:\n  - %s",
			len(problems), strings.Join(problems, "\n  - "))
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

// Helper functions

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

	// A read error (EOF on a redirected stdin) leaves the default in place, which
	// is the behaviour a non-interactive caller wants.
	_, _ = fmt.Scanln(&value) //nolint:errcheck // the default stands on a read error

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
			// The default comes from this package, not from the user; if it does
			// not parse that is a bug here, and zero is the honest answer.
			intValue, _ = strconv.Atoi(defaultValue) //nolint:errcheck // fallback to zero is intended
		}
		return intValue
	case "int64":
		int64Value, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			fmt.Printf("Invalid 64-bit integer %q; using %s\n", value, defaultValue)
			int64Value, _ = strconv.ParseInt(defaultValue, 10, 64) //nolint:errcheck // fallback to zero is intended
		}
		return int64Value
	case "float":
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			fmt.Printf("Invalid number %q; using %s\n", value, defaultValue)
			floatValue, _ = strconv.ParseFloat(defaultValue, 64) //nolint:errcheck // fallback to zero is intended
		}
		return floatValue
	case "bool":
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			fmt.Printf("Invalid boolean %q; using %s\n", value, defaultValue)
			boolValue, _ = strconv.ParseBool(defaultValue) //nolint:errcheck // fallback to false is intended
		}
		return boolValue
	default:
		return value
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

// promptString reads a value from the terminal as a string.
//
// promptValue returns interface{}, and callers asserted `.(string)` directly. The
// assertion panics if the value is ever anything else -- a library taking the
// host process down because of its own internal type dispatch. The comma-ok form
// falls back to the default instead.
func (c *Config) promptStringValue(key, defaultValue string, required bool) string {
	raw := c.promptValue(key, defaultValue, required, "string")
	if value, ok := raw.(string); ok {
		return value
	}
	return defaultValue
}

// PromptYesNo prompts the user with a given question and returns a bool value based on their response.
func (c *Config) PromptYesNo(question string) bool {
	affirmativeResponses := []string{"yes", "y", "true", "t"}
	negativeResponses := []string{"no", "n", "false", "f"}

	for {
		response := strings.ToLower(c.promptStringValue(question, "", true))
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
	walletName = c.promptStringValue("Wallet Name", "", false)
	walletPass = c.promptStringValue("Passphrase", "", true)
	walletTags = c.promptTags()
	return
}

// promptTags prompts the user to enter a comma-delimited list of tags for the wallet.
func (c *Config) promptTags() []string {
	tagsStr := c.promptStringValue("Tags (comma-separated)", "", false)
	tags := strings.Split(tagsStr, ",")
	for i := 0; i < len(tags); i++ {
		tags[i] = strings.TrimSpace(tags[i])
	}
	return tags
}
