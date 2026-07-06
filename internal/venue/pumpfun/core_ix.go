package pumpfun

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

const BuyMinBaseOutOffset uint16 = 16

const (
	BuySpendableQuoteInOffset  uint16 = 8
	SellBaseAmountInOffset     uint16 = 8
)

// BuildBuySetupInstructions returns ATA/setup instructions before a patched buy core ix.
func BuildBuySetupInstructions(p BuildParams, kind QuoteKind) ([]solana.Instruction, error) {
	return BuildBuySetupInstructionsWithPayer(p, kind, p.User)
}

// BuildBuySetupInstructionsWithPayer creates user-owned ATAs with the given rent payer.
func BuildBuySetupInstructionsWithPayer(p BuildParams, kind QuoteKind, payer solana.PublicKey) ([]solana.Instruction, error) {
	switch kind {
	case QuoteNativeSOL:
		createATA, err := createATAIdempotent(payer, p.User, p.BaseMint, p.BaseTokenProgram)
		if err != nil {
			return nil, err
		}
		return []solana.Instruction{createATA}, nil
	case QuoteSPL:
		return appendV2ATASetupWithPayer(nil, p, payer)
	default:
		return nil, fmt.Errorf("unsupported quote kind")
	}
}

// BuildBuyCoreIx returns the single pump buy instruction (no compute budget / ATA setup).
func BuildBuyCoreIx(p BuildParams, kind QuoteKind) (solana.Instruction, error) {
	switch kind {
	case QuoteNativeSOL:
		return buildBuyNativeCoreIx(p)
	case QuoteSPL:
		return buildBuyV2CoreIx(p)
	default:
		return nil, fmt.Errorf("unsupported quote kind")
	}
}

// BuildSellCoreIx returns the single pump sell instruction (no compute budget / fee / close).
func BuildSellCoreIx(p BuildParams, kind QuoteKind) (solana.Instruction, error) {
	switch kind {
	case QuoteNativeSOL:
		return buildSellNativeCoreIx(p)
	case QuoteSPL:
		return buildSellV2CoreIx(p)
	default:
		return nil, fmt.Errorf("unsupported quote kind")
	}
}

func buildBuyNativeCoreIx(p BuildParams) (solana.Instruction, error) {
	ixs, err := BuildBuyInstructions(p)
	if err != nil {
		return nil, err
	}
	return ixs[len(ixs)-1], nil
}

func buildBuyV2CoreIx(p BuildParams) (solana.Instruction, error) {
	accts, err := resolveV2Accounts(p)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 24)
	copy(data[:8], discBuyExactQuoteInV2[:])
	putU64LE(data[8:16], p.SpendableQuoteIn)
	putU64LE(data[16:24], p.MinBaseOut)
	program := ProgramID()
	return solana.NewInstruction(program, solana.AccountMetaSlice{
		{PublicKey: accts.globalPK, IsWritable: false, IsSigner: false},
		{PublicKey: p.BaseMint, IsWritable: false, IsSigner: false},
		{PublicKey: p.QuoteMint, IsWritable: false, IsSigner: false},
		{PublicKey: p.BaseTokenProgram, IsWritable: false, IsSigner: false},
		{PublicKey: p.QuoteTokenProgram, IsWritable: false, IsSigner: false},
		{PublicKey: solana.SPLAssociatedTokenAccountProgramID, IsWritable: false, IsSigner: false},
		{PublicKey: accts.pumpFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocQuoteFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.buybackFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocQuoteBuybackFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.bcPK, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocBC, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocQuoteBC, IsWritable: true, IsSigner: false},
		{PublicKey: p.User, IsWritable: true, IsSigner: true},
		{PublicKey: accts.userBaseATA, IsWritable: true, IsSigner: false},
		{PublicKey: accts.userQuoteATA, IsWritable: true, IsSigner: false},
		{PublicKey: accts.creatorVault, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocCreatorVault, IsWritable: true, IsSigner: false},
		{PublicKey: accts.sharingConfig, IsWritable: false, IsSigner: false},
		{PublicKey: accts.gva, IsWritable: false, IsSigner: false},
		{PublicKey: accts.uva, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocUVA, IsWritable: true, IsSigner: false},
		{PublicKey: accts.feeConfig, IsWritable: false, IsSigner: false},
		{PublicKey: FeeProgramID(), IsWritable: false, IsSigner: false},
		{PublicKey: solana.SystemProgramID, IsWritable: false, IsSigner: false},
		{PublicKey: accts.eventAuth, IsWritable: false, IsSigner: false},
		{PublicKey: program, IsWritable: false, IsSigner: false},
	}, data), nil
}

func buildSellNativeCoreIx(p BuildParams) (solana.Instruction, error) {
	ixs, err := BuildSellCoreInstructions(p)
	if err != nil {
		return nil, err
	}
	for i := len(ixs) - 1; i >= 0; i-- {
		if ixs[i].ProgramID().Equals(ProgramID()) {
			return ixs[i], nil
		}
	}
	return nil, fmt.Errorf("pump sell core ix not found")
}

func buildSellV2CoreIx(p BuildParams) (solana.Instruction, error) {
	accts, err := resolveV2Accounts(p)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 24)
	copy(data[:8], discSellV2[:])
	putU64LE(data[8:16], p.BaseAmountIn)
	putU64LE(data[16:24], p.MinQuoteOut)
	program := ProgramID()
	return solana.NewInstruction(program, solana.AccountMetaSlice{
		{PublicKey: accts.globalPK, IsWritable: false, IsSigner: false},
		{PublicKey: p.BaseMint, IsWritable: false, IsSigner: false},
		{PublicKey: p.QuoteMint, IsWritable: false, IsSigner: false},
		{PublicKey: p.BaseTokenProgram, IsWritable: false, IsSigner: false},
		{PublicKey: p.QuoteTokenProgram, IsWritable: false, IsSigner: false},
		{PublicKey: solana.SPLAssociatedTokenAccountProgramID, IsWritable: false, IsSigner: false},
		{PublicKey: accts.pumpFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocQuoteFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.buybackFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocQuoteBuybackFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: accts.bcPK, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocBC, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocQuoteBC, IsWritable: true, IsSigner: false},
		{PublicKey: p.User, IsWritable: true, IsSigner: true},
		{PublicKey: accts.userBaseATA, IsWritable: true, IsSigner: false},
		{PublicKey: accts.userQuoteATA, IsWritable: true, IsSigner: false},
		{PublicKey: accts.creatorVault, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocCreatorVault, IsWritable: true, IsSigner: false},
		{PublicKey: accts.sharingConfig, IsWritable: false, IsSigner: false},
		{PublicKey: accts.uva, IsWritable: true, IsSigner: false},
		{PublicKey: accts.assocUVA, IsWritable: true, IsSigner: false},
		{PublicKey: accts.feeConfig, IsWritable: false, IsSigner: false},
		{PublicKey: FeeProgramID(), IsWritable: false, IsSigner: false},
		{PublicKey: solana.SystemProgramID, IsWritable: false, IsSigner: false},
		{PublicKey: accts.eventAuth, IsWritable: false, IsSigner: false},
		{PublicKey: program, IsWritable: false, IsSigner: false},
	}, data), nil
}
