package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/logx"
	solpkg "github.com/ifx-run/ifx-launchpad-orchestrator/internal/solana"
)

// Fetcher performs the single snapshot RPC round.
type Fetcher struct {
	cfg    *config.Config
	client *solpkg.Client
}

func NewFetcher(cfg *config.Config, client *solpkg.Client) *Fetcher {
	return &Fetcher{cfg: cfg, client: client}
}

func (f *Fetcher) Fetch(ctx context.Context, plan *FetchPlan) (*ChainSnapshot, error) {
	start := time.Now()
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	commitment := solpkg.SnapshotCommitment(f.cfg.Snapshot.Commitment)
	accounts, err := f.client.FetchAccounts(ctx, plan.Keys, commitment, f.cfg.Snapshot.BatchSize)
	if err != nil {
		logx.Error("snapshot", "fetch failed",
			"err", err,
			"accounts", len(plan.Keys),
			"commitment", f.cfg.Snapshot.Commitment,
			"ms", logx.Since(start),
		)
		return nil, fmt.Errorf("snapshot fetch: %w", err)
	}
	logx.Debug("snapshot", "fetch ok",
		"accounts", len(plan.Keys),
		"commitment", f.cfg.Snapshot.Commitment,
		"ms", logx.Since(start),
	)
	return NewChainSnapshot(plan.Keys, accounts), nil
}
