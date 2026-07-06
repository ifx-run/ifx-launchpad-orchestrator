package solana

import (
	"context"
	"fmt"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	altprog "github.com/gagliardetto/solana-go/programs/address-lookup-table"
)

func (c *Client) LoadAddressLookupTables(ctx context.Context, cfg *config.Config) error {
	tables := make(map[solana.PublicKey]solana.PublicKeySlice)
	for _, addr := range cfg.Solana.AddressLookupTables {
		pk, err := ParsePubkey(addr)
		if err != nil {
			return err
		}
		state, err := altprog.GetAddressLookupTableStateWithOpts(ctx, c.rpc, pk, &rpc.GetAccountInfoOpts{
			Commitment: c.commitment,
		})
		if err != nil {
			return fmt.Errorf("load ALT %s: %w", addr, err)
		}
		if state == nil || !state.IsActive() {
			return fmt.Errorf("ALT %s is missing or deactivated", addr)
		}
		tables[pk] = state.Addresses
	}
	c.altTables = tables
	return nil
}

func (c *Client) AddressLookupTables() map[solana.PublicKey]solana.PublicKeySlice {
	if c.altTables == nil {
		return map[solana.PublicKey]solana.PublicKeySlice{}
	}
	return c.altTables
}
