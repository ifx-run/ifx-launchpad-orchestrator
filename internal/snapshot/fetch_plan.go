package snapshot

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/route"
	solpkg "github.com/chopin65536/ifx-launchpad-orchestrator/internal/solana"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/meteora_dbc"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/raydium_launchpad"
	"github.com/gagliardetto/solana-go"
)

// FetchPlanInput drives pubkey derivation before the single RPC snapshot.
type FetchPlanInput struct {
	PairClass   route.PairClass
	InputMint   string
	OutputMint  string
	UserPubkey  string
	Recipient   string
	BridgePool  *bridge.DiscoveredPool
}

// FetchPlan is the deduplicated pubkey set for one snapshot RPC.
type FetchPlan struct {
	Keys []solana.PublicKey
	Meta FetchPlanInput
}

func BuildFetchPlan(cfg *config.Config, in FetchPlanInput) (*FetchPlan, error) {
	seen := make(map[solana.PublicKey]struct{})
	var keys []solana.PublicKey

	add := func(pk solana.PublicKey) {
		if _, ok := seen[pk]; ok {
			return
		}
		seen[pk] = struct{}{}
		keys = append(keys, pk)
	}

	addMint := func(mintStr string) error {
		mint, err := solpkg.ParsePubkey(mintStr)
		if err != nil {
			return err
		}
		add(mint)
		return nil
	}

	addATAs := func(ownerStr string, mints ...string) error {
		owner, err := solpkg.ParsePubkey(ownerStr)
		if err != nil {
			return err
		}
		for _, m := range mints {
			mint, err := solpkg.ParsePubkey(m)
			if err != nil {
				return err
			}
			pair, err := solpkg.DeriveATAPair(owner, mint)
			if err != nil {
				return err
			}
			add(pair.Legacy)
			add(pair.Token2022)
		}
		return nil
	}

	switch in.PairClass {
	case route.PairSOLSettlement:
		if err := addMint(in.InputMint); err != nil {
			return nil, err
		}
		if err := addATAs(in.UserPubkey, in.InputMint); err != nil {
			return nil, err
		}
	case route.PairQuoteSwap:
		if err := addMint(in.InputMint); err != nil {
			return nil, err
		}
		if err := addMint(in.OutputMint); err != nil {
			return nil, err
		}
		if in.BridgePool != nil {
			poolPK, err := solpkg.ParsePubkey(in.BridgePool.PoolID)
			if err != nil {
				return nil, err
			}
			add(poolPK)
		}
		if err := addATAs(in.UserPubkey, in.InputMint, in.OutputMint); err != nil {
			return nil, err
		}
		if in.Recipient != "" && in.Recipient != in.UserPubkey {
			if err := addATAs(in.Recipient, in.InputMint, in.OutputMint); err != nil {
				return nil, err
			}
		}

	default:
		launchpadMints := []string{nonQuoteMint(cfg, in)}
		if in.PairClass == route.PairSwapLaunchpad {
			launchpadMints = uniqueStrings(in.InputMint, in.OutputMint)
		}
		for _, mintStr := range launchpadMints {
			baseMint, err := solpkg.ParsePubkey(mintStr)
			if err != nil {
				return nil, err
			}
			for _, fn := range []func(*config.Config, solana.PublicKey) ([]solana.PublicKey, error){
				pumpfun.CandidateAccounts,
				raydium_launchpad.CandidateAccounts,
				meteora_dbc.CandidateAccounts,
			} {
				candidates, err := fn(cfg, baseMint)
				if err != nil {
					return nil, err
				}
				for _, pk := range candidates {
					add(pk)
				}
			}
		}

		mints := uniqueStrings(in.InputMint, in.OutputMint, cfg.Quotes.WSOLMint, cfg.Quotes.USDCMint, cfg.Quotes.USDTMint)
		for _, m := range mints {
			if err := addMint(m); err != nil {
				return nil, err
			}
		}

		if in.BridgePool != nil {
			poolPK, err := solpkg.ParsePubkey(in.BridgePool.PoolID)
			if err != nil {
				return nil, err
			}
			add(poolPK)
		}

		if err := addATAs(in.UserPubkey, mints...); err != nil {
			return nil, err
		}
		recipient := in.Recipient
		if recipient == "" {
			recipient = in.UserPubkey
		}
		if recipient != in.UserPubkey {
			if err := addATAs(recipient, mints...); err != nil {
				return nil, err
			}
		}
	}

	return &FetchPlan{Keys: keys, Meta: in}, nil
}

func nonQuoteMint(cfg *config.Config, in FetchPlanInput) string {
	if !cfg.IsQuoteMint(in.InputMint) {
		return in.InputMint
	}
	return in.OutputMint
}

func uniqueStrings(items ...string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range items {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func (p *FetchPlan) Validate() error {
	if len(p.Keys) == 0 {
		return fmt.Errorf("fetch plan has no keys")
	}
	return nil
}
