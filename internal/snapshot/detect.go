package snapshot

import (
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/meteora_dbc"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/raydium_launchpad"
	"github.com/gagliardetto/solana-go"
)

type venueCandidate struct {
	id  venue.ID
	pda solana.PublicKey
}

// DetectLaunchpadVenue picks venue from which pool account exists on-chain.
func DetectLaunchpadVenue(cfg *config.Config, snap *ChainSnapshot, baseMint solana.PublicKey) (*venue.Detection, error) {
	var hits []venueCandidate

	check := func(id venue.ID, keys []solana.PublicKey, err error) error {
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}
		if snap.HasAccount(keys[0]) {
			hits = append(hits, venueCandidate{id: id, pda: keys[0]})
		}
		return nil
	}

	if keys, err := pumpfun.CandidateAccounts(cfg, baseMint); err != nil {
		return nil, err
	} else if err := check(venue.IDPumpfun, keys, nil); err != nil {
		return nil, err
	}
	if keys, err := raydium_launchpad.CandidateAccounts(cfg, baseMint); err != nil {
		return nil, err
	} else if err := check(venue.IDRaydiumLaunchpad, keys, nil); err != nil {
		return nil, err
	}
	if keys, err := meteora_dbc.CandidateAccounts(cfg, baseMint); err != nil {
		return nil, err
	} else if err := check(venue.IDMeteoraDBC, keys, nil); err != nil {
		return nil, err
	}

	if len(hits) == 0 {
		return nil, fmt.Errorf("not a supported launchpad token")
	}

	chosen := pickByPriority(cfg, hits)
	return &venue.Detection{
		Venue:    chosen.id,
		BaseMint: baseMint.String(),
		PoolKey:  chosen.pda.String(),
	}, nil
}

func pickByPriority(cfg *config.Config, hits []venueCandidate) venueCandidate {
	priority := cfg.Detect.VenuePriority
	for _, name := range priority {
		for _, h := range hits {
			if h.id.String() == name {
				return h
			}
		}
	}
	return hits[0]
}

// MintOwner returns the owner program of a mint from snapshot.
func MintOwner(snap *ChainSnapshot, mint solana.PublicKey) (solana.PublicKey, bool) {
	acct, ok := snap.Accounts[mint]
	if !ok || acct == nil {
		return solana.PublicKey{}, false
	}
	return acct.Owner, true
}
