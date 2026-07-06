package pumpfun

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
)

type v2AccountSet struct {
	globalPK                         solana.PublicKey
	bcPK                             solana.PublicKey
	assocBC                          solana.PublicKey
	assocQuoteBC                     solana.PublicKey
	userBaseATA                      solana.PublicKey
	userQuoteATA                     solana.PublicKey
	creatorVault                     solana.PublicKey
	assocCreatorVault                solana.PublicKey
	sharingConfig                    solana.PublicKey
	gva                              solana.PublicKey
	uva                              solana.PublicKey
	assocUVA                         solana.PublicKey
	feeConfig                        solana.PublicKey
	eventAuth                        solana.PublicKey
	pumpFeeRecipient                 solana.PublicKey
	assocQuoteFeeRecipient           solana.PublicKey
	buybackFeeRecipient              solana.PublicKey
	assocQuoteBuybackFeeRecipient    solana.PublicKey
}

func resolveV2Accounts(p BuildParams) (v2AccountSet, error) {
	program := ProgramID()
	globalPK, err := GlobalPDAFromProgram(program)
	if err != nil {
		return v2AccountSet{}, err
	}
	bcPK, err := bondingCurvePDAFromProgram(program, p.BaseMint)
	if err != nil {
		return v2AccountSet{}, err
	}
	if p.QuoteMint.IsZero() {
		return v2AccountSet{}, fmt.Errorf("quote mint required for v2 instructions")
	}
	if p.QuoteTokenProgram.IsZero() {
		return v2AccountSet{}, fmt.Errorf("quote token program required for v2 instructions")
	}

	assocBC, err := ataAddress(bcPK, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}
	assocQuoteBC, err := ataAddress(bcPK, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}
	userBaseATA, err := ataAddress(p.User, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}
	userQuoteATA, err := ataAddress(p.User, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}
	creatorVault, err := CreatorVaultPDA(p.Curve.Creator)
	if err != nil {
		return v2AccountSet{}, err
	}
	assocCreatorVault, err := ataAddress(creatorVault, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}
	sharingConfig, err := SharingConfigPDA(p.BaseMint)
	if err != nil {
		return v2AccountSet{}, err
	}
	gva, err := GlobalVolumeAccumulatorPDA()
	if err != nil {
		return v2AccountSet{}, err
	}
	uva, err := UserVolumeAccumulatorPDA(p.User)
	if err != nil {
		return v2AccountSet{}, err
	}
	assocUVA, err := ataAddress(uva, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}
	feeConfig, err := FeeConfigPDA()
	if err != nil {
		return v2AccountSet{}, err
	}
	eventAuth, err := EventAuthorityPDA()
	if err != nil {
		return v2AccountSet{}, err
	}
	pumpFeeRecipient := PickFeeRecipient(p.Curve.IsMayhemMode)
	assocQuoteFeeRecipient, err := ataAddress(pumpFeeRecipient, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}
	buybackFeeRecipient := PickBuybackFeeRecipient()
	assocQuoteBuybackFeeRecipient, err := ataAddress(buybackFeeRecipient, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return v2AccountSet{}, err
	}

	return v2AccountSet{
		globalPK:                      globalPK,
		bcPK:                          bcPK,
		assocBC:                       assocBC,
		assocQuoteBC:                  assocQuoteBC,
		userBaseATA:                   userBaseATA,
		userQuoteATA:                  userQuoteATA,
		creatorVault:                  creatorVault,
		assocCreatorVault:             assocCreatorVault,
		sharingConfig:                 sharingConfig,
		gva:                           gva,
		uva:                           uva,
		assocUVA:                      assocUVA,
		feeConfig:                     feeConfig,
		eventAuth:                     eventAuth,
		pumpFeeRecipient:              pumpFeeRecipient,
		assocQuoteFeeRecipient:        assocQuoteFeeRecipient,
		buybackFeeRecipient:           buybackFeeRecipient,
		assocQuoteBuybackFeeRecipient: assocQuoteBuybackFeeRecipient,
	}, nil
}

func appendV2ATASetup(ixs []solana.Instruction, p BuildParams) ([]solana.Instruction, error) {
	return appendV2ATASetupWithPayer(ixs, p, p.User)
}

func appendV2ATASetupWithPayer(ixs []solana.Instruction, p BuildParams, payer solana.PublicKey) ([]solana.Instruction, error) {
	createBase, err := createATAIdempotent(payer, p.User, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return nil, err
	}
	ixs = append(ixs, createBase)
	createQuote, err := createATAIdempotent(payer, p.User, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return nil, err
	}
	ixs = append(ixs, createQuote)
	return ixs, nil
}

func appendQuoteServiceFee(ixs []solana.Instruction, p BuildParams) []solana.Instruction {
	if p.ServiceFeeQuote == 0 || p.PlatformFeeQuoteATA.IsZero() {
		return ixs
	}
	userQuoteATA, _ := ataAddress(p.User, p.QuoteMint, p.QuoteTokenProgram)
	ixs = append(ixs, token.NewTransferCheckedInstruction(
		p.ServiceFeeQuote,
		quoteDecimalsForMint(p.QuoteMint),
		userQuoteATA,
		p.QuoteMint,
		p.PlatformFeeQuoteATA,
		p.User,
		[]solana.PublicKey{},
	).Build())
	return ixs
}

func quoteDecimalsForMint(mint solana.PublicKey) uint8 {
	// WSOL and most pump quote mints: 9 for SOL, 6 for stables; caller paths use curve quote.
	wsol := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	if mint.Equals(wsol) {
		return 9
	}
	return 6
}

// BuildBuyV2Instructions builds buy_exact_quote_in_v2 for SPL quote pools (e.g. USDC).
func BuildBuyV2Instructions(p BuildParams) ([]solana.Instruction, error) {
	accts, err := resolveV2Accounts(p)
	if err != nil {
		return nil, err
	}

	var ixs []solana.Instruction
	ixs = append(ixs,
		computebudget.NewSetComputeUnitLimitInstruction(p.ComputeUnitLimit).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(p.ComputeUnitPrice).Build(),
	)
	ixs, err = appendV2ATASetup(ixs, p)
	if err != nil {
		return nil, err
	}
	ixs = appendQuoteServiceFee(ixs, p)

	data := make([]byte, 24)
	copy(data[:8], discBuyExactQuoteInV2[:])
	putU64LE(data[8:16], p.SpendableQuoteIn)
	putU64LE(data[16:24], p.MinBaseOut)

	program := ProgramID()
	ixs = append(ixs, solana.NewInstruction(program, solana.AccountMetaSlice{
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
	}, data))
	return ixs, nil
}

// BuildSellV2Instructions builds sell_v2 for SPL quote pools.
func BuildSellV2Instructions(p BuildParams) ([]solana.Instruction, error) {
	accts, err := resolveV2Accounts(p)
	if err != nil {
		return nil, err
	}

	var ixs []solana.Instruction
	ixs = append(ixs,
		computebudget.NewSetComputeUnitLimitInstruction(p.ComputeUnitLimit).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(p.ComputeUnitPrice).Build(),
	)

	data := make([]byte, 24)
	copy(data[:8], discSellV2[:])
	putU64LE(data[8:16], p.BaseAmountIn)
	putU64LE(data[16:24], p.MinQuoteOut)

	program := ProgramID()
	ixs = append(ixs, solana.NewInstruction(program, solana.AccountMetaSlice{
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
	}, data))

	if p.ServiceFeeQuote > 0 && !p.PlatformFeeQuoteATA.IsZero() {
		userQuoteATA, _ := ataAddress(p.User, p.QuoteMint, p.QuoteTokenProgram)
		ixs = append(ixs, token.NewTransferCheckedInstruction(
			p.ServiceFeeQuote,
			quoteDecimalsForMint(p.QuoteMint),
			userQuoteATA,
			p.QuoteMint,
			p.PlatformFeeQuoteATA,
			p.User,
			[]solana.PublicKey{},
		).Build())
	}

	return ixs, nil
}
