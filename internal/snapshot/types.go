package snapshot

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// ChainSnapshot is the in-memory view after a single getMultipleAccounts round.
type ChainSnapshot struct {
	Accounts map[solana.PublicKey]*rpc.Account
	Keys     []solana.PublicKey
}

func NewChainSnapshot(keys []solana.PublicKey, accounts map[solana.PublicKey]*rpc.Account) *ChainSnapshot {
	return &ChainSnapshot{
		Accounts: accounts,
		Keys:     keys,
	}
}

func (s *ChainSnapshot) HasAccount(pk solana.PublicKey) bool {
	acct, ok := s.Accounts[pk]
	return ok && acct != nil && acct.Data != nil
}
