package ifx

import (
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/venue/pumpfun"
	"github.com/gagliardetto/solana-go"
	"github.com/ifx-run/ifx/go-sdk/patch"
)

// PumpBuySponsoredParams builds a patched SOL pump buy with sponsor repay from spendable SOL.
type PumpBuySponsoredParams struct {
	BuyTemplate      solana.Instruction
	SpendableQuoteIn uint64 // lamports for pump after platform fee
	User             solana.PublicKey
	SponsorPayer     solana.PublicKey
	ATACreates       []ATASetupSpec
	Repay            SponsoredRepayParams
	FixedRepayCost   uint64 // basic + priority + tip (rent measured on-chain)
}

// PlanPumpBuySponsored returns reset → sponsor ATAs → repay from spendable SOL → patched buy.
func PlanPumpBuySponsored(cfg *config.Config, p PumpBuySponsoredParams) ([]solana.Instruction, error) {
	s, err := NewScratch(cfg)
	if err != nil {
		return nil, err
	}
	out := []solana.Instruction{s.IxReset()}

	out, ataCost, err := AppendSponsorATACreates(s, out, p.SponsorPayer, p.ATACreates)
	if err != nil {
		return nil, err
	}

	b := s.LetBuilder()
	spendConst, err := b.LetConstU64(p.SpendableQuoteIn)
	if err != nil {
		return nil, err
	}
	bIx, err := b.BuildIx()
	if err != nil {
		return nil, err
	}
	out = append(out, bIx)

	p.Repay.FixedCostLamports = p.FixedRepayCost
	out, netBuy, err := AppendRepayDeductNet(s, out, p.Repay, spendConst, ataCost, p.FixedRepayCost)
	if err != nil {
		return nil, err
	}

	buyCpi, err := rawCpiIxScratch(s, p.BuyTemplate, patch.RawCpiPatch(pumpfun.BuySpendableQuoteInOffset, netBuy))
	if err != nil {
		return nil, err
	}
	out = append(out, buyCpi)
	return out, nil
}
