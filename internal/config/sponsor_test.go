package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSponsorRepayPubkeyDefaultsToPubkey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	if err := os.WriteFile(path, []byte(`
[solana]
rpc_url = "https://example.invalid"

[quotes]
wsol_mint = "So11111111111111111111111111111111111111112"
usdc_mint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

[sponsor]
pubkey = "2nYsdZrpKCzSQXhkyMtX2XnqkoyPwGeVcj3vgNkTJJ4J"
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sponsor.RepayPubkey != cfg.Sponsor.Pubkey {
		t.Fatalf("repay_pubkey = %q, want %q", cfg.Sponsor.RepayPubkey, cfg.Sponsor.Pubkey)
	}
}

func TestSponsorRepayPubkeyDistinct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.toml")
	if err := os.WriteFile(path, []byte(`
[solana]
rpc_url = "https://example.invalid"

[quotes]
wsol_mint = "So11111111111111111111111111111111111111112"
usdc_mint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

[sponsor]
pubkey = "2nYsdZrpKCzSQXhkyMtX2XnqkoyPwGeVcj3vgNkTJJ4J"
repay_pubkey = "BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P"
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sponsor.RepayPubkey != "BKNnVDyzcPGCWnk8zX3Cn2KKhLASk5iTjVpxUW7YTb8P" {
		t.Fatalf("repay_pubkey = %q", cfg.Sponsor.RepayPubkey)
	}
}
