package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
)

const splATARentLamports = 2_039_280

// SponsorSigner holds the sponsor fee-payer key for partial co-sign.
type SponsorSigner struct {
	Pubkey solana.PublicKey
	Key    solana.PrivateKey
}

func loadSponsorSigner(cfg *config.Config) (*SponsorSigner, error) {
	if !cfg.Sponsor.Enabled {
		return nil, fmt.Errorf("sponsor disabled")
	}
	if cfg.Sponsor.KeypairPath == "" {
		return nil, fmt.Errorf("sponsor.keypair_path required")
	}
	raw, err := os.ReadFile(cfg.Sponsor.KeypairPath)
	if err != nil {
		return nil, fmt.Errorf("read sponsor keypair: %w", err)
	}
	var secret []byte
	if err := json.Unmarshal(raw, &secret); err != nil {
		return nil, fmt.Errorf("parse sponsor keypair json: %w", err)
	}
	key := solana.PrivateKey(secret)
	pubkey, err := solana.PublicKeyFromBase58(cfg.Sponsor.Pubkey)
	if err != nil {
		return nil, err
	}
	if !key.PublicKey().Equals(pubkey) {
		return nil, fmt.Errorf("sponsor pubkey does not match keypair")
	}
	return &SponsorSigner{Pubkey: pubkey, Key: key}, nil
}

func (s *Service) sponsorPubkey() (solana.PublicKey, error) {
	if !s.cfg.Sponsor.Enabled {
		return solana.PublicKey{}, fmt.Errorf("sponsor disabled")
	}
	return solana.PublicKeyFromBase58(s.cfg.Sponsor.Pubkey)
}

// ataPayerForMode returns who pays ATA rent and tx fees for the variant.
func (s *Service) ataPayerForMode(mode VariantMode, user solana.PublicKey) (solana.PublicKey, error) {
	if mode.Sponsored {
		return s.sponsorPubkey()
	}
	return user, nil
}

func (s *Service) sponsorRepayPubkey() (solana.PublicKey, error) {
	if !s.cfg.Sponsor.Enabled {
		return solana.PublicKey{}, fmt.Errorf("sponsor disabled")
	}
	repay := s.cfg.Sponsor.RepayPubkey
	if repay == "" {
		repay = s.cfg.Sponsor.Pubkey
	}
	return solana.PublicKeyFromBase58(repay)
}

// EstimateRepayLamports covers basic sig fee, priority fee, ATA rent, Jito tip, and buffer.
func EstimateRepayLamports(cfg *config.Config, tier config.PriorityFeeTier, numSignatures, numATACreated int, jitoTip uint64) uint64 {
	const baseSigFee = 5_000
	basic := uint64(numSignatures) * baseSigFee
	priority := uint64(tier.ComputeUnitLimit) * tier.MicroLamports / 1_000_000
	rent := uint64(numATACreated) * splATARentLamports
	subtotal := basic + priority + rent + jitoTip
	if cfg.Sponsor.RepayBufferPercent == 0 {
		return subtotal
	}
	return subtotal + subtotal*uint64(cfg.Sponsor.RepayBufferPercent)/100
}
