package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	defaultConfigPath = "config.toml"
	envConfigPath     = "IFX_LAUNCHPAD_CONFIG"
)

type Config struct {
	Server      ServerConfig      `toml:"server"`
	Solana      SolanaConfig      `toml:"solana"`
	Snapshot    SnapshotConfig    `toml:"snapshot"`
	Ifx         IfxConfig         `toml:"ifx"`
	Quotes      QuotesConfig      `toml:"quotes"`
	Venues      VenuesConfig      `toml:"venues"`
	Bridge      BridgeConfig      `toml:"bridge"`
	Jupiter     JupiterConfig     `toml:"jupiter"`
	Jito        JitoConfig        `toml:"jito"`
	PriorityFee PriorityFeeConfig `toml:"priority_fee"`
	ServiceFee  ServiceFeeConfig  `toml:"service_fee"`
	Sponsor     SponsorConfig     `toml:"sponsor"`
	Quote       QuoteConfig       `toml:"quote"`
	Tx          TxConfig          `toml:"tx"`
	Detect      DetectConfig      `toml:"detect"`

	sourcePath string
}

type ServerConfig struct {
	Host  string `toml:"host"`
	Port  int    `toml:"port"`
	Debug bool   `toml:"debug"`
}

type SolanaConfig struct {
	RPCURL              string   `toml:"rpc_url"`
	Commitment          string   `toml:"commitment"`
	AddressLookupTables []string `toml:"address_lookup_tables"`
}

type SnapshotConfig struct {
	Commitment string `toml:"commitment"`
	BatchSize  int    `toml:"batch_size"`
}

type IfxConfig struct {
	ProgramID    string   `toml:"program_id"`
	PublicFrames []string `toml:"public_frames"`
}

type QuotesConfig struct {
	WSOLMint string `toml:"wsol_mint"`
	USDCMint string `toml:"usdc_mint"`
	USDTMint string `toml:"usdt_mint"`
}

type VenuesConfig struct {
	Pump             VenueProgramConfig `toml:"pump"`
	RaydiumLaunchpad VenueProgramConfig `toml:"raydium_launchpad"`
	MeteoraDBC       VenueProgramConfig `toml:"meteora_dbc"`
}

type VenueProgramConfig struct {
	ProgramID string `toml:"program_id"`
	Global    string `toml:"global"`
}

type BridgeConfig struct {
	SupportedTypes   []string             `toml:"supported_types"`
	MaxSwapAccounts  int                  `toml:"max_swap_accounts"`
	OnlyDirectRoutes bool                 `toml:"only_direct_routes"`
	RaydiumAMMv4     BridgeProgramConfig  `toml:"raydium_amm_v4"`
	RaydiumCPMM      BridgeProgramConfig  `toml:"raydium_cpmm"`
	MeteoraDAMMv2    BridgeProgramConfig  `toml:"meteora_damm_v2"`
	FallbackPools    []FallbackPoolConfig `toml:"fallback_pools"`
}

type BridgeProgramConfig struct {
	ProgramID      string `toml:"program_id"`
	SwapIxAccounts int    `toml:"swap_ix_accounts"`
}

type FallbackPoolConfig struct {
	PoolType   string `toml:"pool_type"`
	PoolID     string `toml:"pool_id"`
	InputMint  string `toml:"input_mint"`
	OutputMint string `toml:"output_mint"`
}

type JupiterConfig struct {
	APIURL         string `toml:"api_url"`
	Enabled        bool   `toml:"enabled"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
	// Optional: required when api_url is https://api.jup.ag/swap/v1 (not needed for lite-api)
	APIKey string `toml:"api_key"`
	// Optional HTTP(S) proxy for Jupiter quote requests, e.g. http://127.0.0.1:7890
	ProxyURL string `toml:"proxy_url"`
}

type JitoConfig struct {
	Enabled     bool   `toml:"enabled"`
	TipAccount  string `toml:"tip_account"`
	TipLamports uint64 `toml:"tip_lamports"`
}

type PriorityFeeTier struct {
	MicroLamports    uint64 `toml:"micro_lamports"`
	ComputeUnitLimit uint32 `toml:"compute_unit_limit"`
}

type PriorityFeeConfig struct {
	DefaultTier string          `toml:"default_tier"`
	Low         PriorityFeeTier `toml:"low"`
	Medium      PriorityFeeTier `toml:"medium"`
	High        PriorityFeeTier `toml:"high"`
}

type ServiceFeeConfig struct {
	BPS     uint16 `toml:"bps"`
	Pubkey  string `toml:"pubkey"`
	USDCATA string `toml:"usdc_ata"`
	USDTATA string `toml:"usdt_ata"`
}

type SponsorConfig struct {
	Enabled            bool   `toml:"enabled"`
	Pubkey             string `toml:"pubkey"`              // fee payer; must match keypair_path
	RepayPubkey        string `toml:"repay_pubkey"`        // gas repay recipient; defaults to pubkey
	KeypairPath        string `toml:"keypair_path"`
	RepayBufferPercent uint16 `toml:"repay_buffer_percent"`
}

type QuoteConfig struct {
	DebounceMS       int `toml:"debounce_ms"`
	DefaultSlippageBPS int `toml:"default_slippage_bps"`
	RPCCacheTTLMS    int `toml:"rpc_cache_ttl_ms"`
}

type TxConfig struct {
	MaxBytes int `toml:"max_bytes"`
}

type DetectConfig struct {
	VenuePriority []string `toml:"venue_priority"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv(envConfigPath)
	}
	if path == "" {
		if _, err := os.Stat(defaultConfigPath); err == nil {
			path = defaultConfigPath
		} else {
			path = "config.example.toml"
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}

	abs, _ := filepath.Abs(path)
	cfg.sourcePath = abs

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8789
	}
	if c.Solana.Commitment == "" {
		c.Solana.Commitment = "confirmed"
	}
	if c.Snapshot.Commitment == "" {
		c.Snapshot.Commitment = "processed"
	}
	if c.Snapshot.BatchSize == 0 {
		c.Snapshot.BatchSize = 100
	}
	if c.Bridge.MaxSwapAccounts == 0 {
		c.Bridge.MaxSwapAccounts = 14
	}
	if c.Bridge.MeteoraDAMMv2.ProgramID == "" {
		c.Bridge.MeteoraDAMMv2.ProgramID = "cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG"
	}
	if c.Bridge.MeteoraDAMMv2.SwapIxAccounts == 0 {
		c.Bridge.MeteoraDAMMv2.SwapIxAccounts = 14
	}
	if c.Jupiter.APIURL == "" {
		c.Jupiter.APIURL = "https://lite-api.jup.ag/swap/v1"
	}
	if c.Jupiter.TimeoutSeconds == 0 {
		c.Jupiter.TimeoutSeconds = 30
	}
	if c.Quote.DefaultSlippageBPS == 0 {
		c.Quote.DefaultSlippageBPS = 100
	}
	if c.Tx.MaxBytes == 0 {
		c.Tx.MaxBytes = 1232
	}
	if c.Solana.RPCURL == "" {
		c.Solana.RPCURL = os.Getenv("SOLANA_RPC_URL")
	}
	if len(c.Detect.VenuePriority) == 0 {
		c.Detect.VenuePriority = []string{"pumpfun", "raydium_launchpad", "meteora_dbc"}
	}
	if c.Sponsor.RepayPubkey == "" {
		c.Sponsor.RepayPubkey = c.Sponsor.Pubkey
	}
}

func (c *Config) validate() error {
	if c.Solana.RPCURL == "" {
		return fmt.Errorf("solana.rpc_url is required (set in local config.toml)")
	}
	if c.Quotes.WSOLMint == "" || c.Quotes.USDCMint == "" {
		return fmt.Errorf("quotes.wsol_mint and quotes.usdc_mint are required")
	}
	return nil
}

func (c *Config) SourcePath() string { return c.sourcePath }

func (c *Config) QuoteMints() []string {
	mints := []string{c.Quotes.WSOLMint, c.Quotes.USDCMint}
	if c.Quotes.USDTMint != "" {
		mints = append(mints, c.Quotes.USDTMint)
	}
	return mints
}

func (c *Config) IsQuoteMint(mint string) bool {
	for _, q := range c.QuoteMints() {
		if q == mint {
			return true
		}
	}
	return false
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func (c *Config) Tier(name string) PriorityFeeTier {
	switch name {
	case "low":
		return c.PriorityFee.Low
	case "high":
		return c.PriorityFee.High
	default:
		return c.PriorityFee.Medium
	}
}
