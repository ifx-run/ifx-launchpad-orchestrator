package solana

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go/rpc"
)

type Blockhash struct {
	Hash       string
	LastValidBlockHeight uint64
}

func (c *Client) LatestBlockhash(ctx context.Context) (Blockhash, error) {
	resp, err := c.rpc.GetLatestBlockhash(ctx, c.commitment)
	if err != nil {
		return Blockhash{}, fmt.Errorf("getLatestBlockhash: %w", err)
	}
	return Blockhash{
		Hash:                 resp.Value.Blockhash.String(),
		LastValidBlockHeight: resp.Value.LastValidBlockHeight,
	}, nil
}

func SnapshotCommitment(name string) rpc.CommitmentType {
	switch name {
	case "confirmed":
		return rpc.CommitmentConfirmed
	case "finalized":
		return rpc.CommitmentFinalized
	default:
		return rpc.CommitmentProcessed
	}
}
