package solana

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
)

type Client struct {
	rpc        *rpc.Client
	commitment rpc.CommitmentType
	altTables  map[solana.PublicKey]solana.PublicKeySlice
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		rpc:        rpc.New(cfg.Solana.RPCURL),
		commitment: parseCommitment(cfg.Solana.Commitment),
	}
}

func (c *Client) RPC() *rpc.Client { return c.rpc }

func (c *Client) Commitment() rpc.CommitmentType { return c.commitment }

func parseCommitment(s string) rpc.CommitmentType {
	switch s {
	case "processed":
		return rpc.CommitmentProcessed
	case "finalized":
		return rpc.CommitmentFinalized
	default:
		return rpc.CommitmentConfirmed
	}
}

func ParsePubkey(s string) (solana.PublicKey, error) {
	pk, err := solana.PublicKeyFromBase58(s)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("invalid pubkey %q: %w", s, err)
	}
	return pk, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.rpc.GetSlot(ctx, c.commitment)
	return err
}
