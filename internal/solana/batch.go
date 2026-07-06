package solana

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// AccountEntry holds one fetched on-chain account.
type AccountEntry struct {
	Pubkey  solana.PublicKey
	Account *rpc.Account
}

// FetchAccounts loads accounts in batches using getMultipleAccounts.
func (c *Client) FetchAccounts(ctx context.Context, keys []solana.PublicKey, commitment rpc.CommitmentType, batchSize int) (map[solana.PublicKey]*rpc.Account, error) {
	if len(keys) == 0 {
		return map[solana.PublicKey]*rpc.Account{}, nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	out := make(map[solana.PublicKey]*rpc.Account, len(keys))
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		chunk := keys[i:end]
		resp, err := c.rpc.GetMultipleAccountsWithOpts(ctx, chunk, &rpc.GetMultipleAccountsOpts{
			Commitment: commitment,
		})
		if err != nil {
			return nil, fmt.Errorf("getMultipleAccounts batch %d-%d: %w", i, end, err)
		}
		if len(resp.Value) != len(chunk) {
			return nil, fmt.Errorf("getMultipleAccounts: expected %d accounts, got %d", len(chunk), len(resp.Value))
		}
		for j, acct := range resp.Value {
			if acct != nil {
				out[chunk[j]] = acct
			}
		}
	}
	return out, nil
}
