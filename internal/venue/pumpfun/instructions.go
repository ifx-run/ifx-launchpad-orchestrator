package pumpfun

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
)

type BuildParams struct {
	Curve              BondingCurve
	BaseMint           solana.PublicKey
	User               solana.PublicKey
	BaseTokenProgram   solana.PublicKey
	QuoteMint          solana.PublicKey
	QuoteTokenProgram  solana.PublicKey
	CashbackEnabled    bool
	SpendableQuoteIn   uint64
	MinBaseOut         uint64
	BaseAmountIn       uint64
	MinQuoteOut        uint64
	ServiceFeeLamports uint64
	ServiceFeeQuote    uint64 // SPL quote fee (USDC/USDT pools)
	PlatformFeePubkey  solana.PublicKey
	PlatformFeeQuoteATA solana.PublicKey
	ComputeUnitLimit   uint32
	ComputeUnitPrice   uint64
}

func createATAIdempotent(payer, owner, mint, tokenProgram solana.PublicKey) (solana.Instruction, error) {
	ata, err := ataAddress(owner, mint, tokenProgram)
	if err != nil {
		return nil, err
	}
	return solana.NewInstruction(
		solana.SPLAssociatedTokenAccountProgramID,
		solana.AccountMetaSlice{
			{PublicKey: payer, IsWritable: true, IsSigner: true},
			{PublicKey: ata, IsWritable: true, IsSigner: false},
			{PublicKey: owner, IsWritable: false, IsSigner: false},
			{PublicKey: mint, IsWritable: false, IsSigner: false},
			{PublicKey: solana.SystemProgramID, IsWritable: false, IsSigner: false},
			{PublicKey: tokenProgram, IsWritable: false, IsSigner: false},
		},
		[]byte{1},
	), nil
}

func ataAddress(owner, mint, tokenProgram solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{owner.Bytes(), tokenProgram.Bytes(), mint.Bytes()},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	return pda, err
}

func BuildBuyInstructions(p BuildParams) ([]solana.Instruction, error) {
	program := ProgramID()
	globalPK, err := GlobalPDAFromProgram(program)
	if err != nil {
		return nil, err
	}
	bcPK, err := bondingCurvePDAFromProgram(program, p.BaseMint)
	if err != nil {
		return nil, err
	}
	assocBC, err := ataAddress(bcPK, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return nil, err
	}
	userBaseATA, err := ataAddress(p.User, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return nil, err
	}

	var ixs []solana.Instruction
	ixs = append(ixs,
		computebudget.NewSetComputeUnitLimitInstruction(p.ComputeUnitLimit).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(p.ComputeUnitPrice).Build(),
	)
	createATA, err := createATAIdempotent(p.User, p.User, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return nil, err
	}
	ixs = append(ixs, createATA)

	if p.ServiceFeeLamports > 0 {
		ixs = append(ixs, system.NewTransferInstruction(
			p.ServiceFeeLamports,
			p.User,
			p.PlatformFeePubkey,
		).Build())
	}

	creatorVault, err := CreatorVaultPDA(p.Curve.Creator)
	if err != nil {
		return nil, err
	}
	eventAuth, err := EventAuthorityPDA()
	if err != nil {
		return nil, err
	}
	gva, err := GlobalVolumeAccumulatorPDA()
	if err != nil {
		return nil, err
	}
	uva, err := UserVolumeAccumulatorPDA(p.User)
	if err != nil {
		return nil, err
	}
	feeConfig, err := FeeConfigPDA()
	if err != nil {
		return nil, err
	}
	pumpFeeRecipient := PickFeeRecipient(p.Curve.IsMayhemMode)
	buybackFeeRecipient := PickBuybackFeeRecipient()
	bcV2, err := BondingCurveV2PDA(p.BaseMint)
	if err != nil {
		return nil, err
	}

	data := make([]byte, 24)
	copy(data[:8], discBuyExactSolIn[:])
	putU64LE(data[8:16], p.SpendableQuoteIn)
	putU64LE(data[16:24], p.MinBaseOut)

	ixs = append(ixs, solana.NewInstruction(program, solana.AccountMetaSlice{
		{PublicKey: globalPK, IsWritable: false, IsSigner: false},
		{PublicKey: pumpFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: p.BaseMint, IsWritable: false, IsSigner: false},
		{PublicKey: bcPK, IsWritable: true, IsSigner: false},
		{PublicKey: assocBC, IsWritable: true, IsSigner: false},
		{PublicKey: userBaseATA, IsWritable: true, IsSigner: false},
		{PublicKey: p.User, IsWritable: true, IsSigner: true},
		{PublicKey: solana.SystemProgramID, IsWritable: false, IsSigner: false},
		{PublicKey: p.BaseTokenProgram, IsWritable: false, IsSigner: false},
		{PublicKey: creatorVault, IsWritable: true, IsSigner: false},
		{PublicKey: eventAuth, IsWritable: false, IsSigner: false},
		{PublicKey: program, IsWritable: false, IsSigner: false},
		{PublicKey: gva, IsWritable: false, IsSigner: false},
		{PublicKey: uva, IsWritable: true, IsSigner: false},
		{PublicKey: feeConfig, IsWritable: false, IsSigner: false},
		{PublicKey: FeeProgramID(), IsWritable: false, IsSigner: false},
		{PublicKey: bcV2, IsWritable: false, IsSigner: false},
		{PublicKey: buybackFeeRecipient, IsWritable: true, IsSigner: false},
	}, data))
	return ixs, nil
}

func BuildSellInstructions(p BuildParams) ([]solana.Instruction, error) {
	return BuildSellCoreInstructions(p)
}

func BuildSellCoreInstructions(p BuildParams) ([]solana.Instruction, error) {
	program := ProgramID()
	globalPK, err := GlobalPDAFromProgram(program)
	if err != nil {
		return nil, err
	}
	bcPK, err := bondingCurvePDAFromProgram(program, p.BaseMint)
	if err != nil {
		return nil, err
	}
	assocBC, err := ataAddress(bcPK, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return nil, err
	}
	userBaseATA, err := ataAddress(p.User, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return nil, err
	}

	creatorVault, err := CreatorVaultPDA(p.Curve.Creator)
	if err != nil {
		return nil, err
	}
	eventAuth, err := EventAuthorityPDA()
	if err != nil {
		return nil, err
	}
	feeConfig, err := FeeConfigPDA()
	if err != nil {
		return nil, err
	}
	pumpFeeRecipient := PickFeeRecipient(p.Curve.IsMayhemMode)
	buybackFeeRecipient := PickBuybackFeeRecipient()
	bcV2, err := BondingCurveV2PDA(p.BaseMint)
	if err != nil {
		return nil, err
	}

	data := make([]byte, 24)
	copy(data[:8], discSell[:])
	putU64LE(data[8:16], p.BaseAmountIn)
	putU64LE(data[16:24], p.MinQuoteOut)

	var ixs []solana.Instruction
	ixs = append(ixs,
		computebudget.NewSetComputeUnitLimitInstruction(p.ComputeUnitLimit).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(p.ComputeUnitPrice).Build(),
	)

	sellAccounts := solana.AccountMetaSlice{
		{PublicKey: globalPK, IsWritable: false, IsSigner: false},
		{PublicKey: pumpFeeRecipient, IsWritable: true, IsSigner: false},
		{PublicKey: p.BaseMint, IsWritable: false, IsSigner: false},
		{PublicKey: bcPK, IsWritable: true, IsSigner: false},
		{PublicKey: assocBC, IsWritable: true, IsSigner: false},
		{PublicKey: userBaseATA, IsWritable: true, IsSigner: false},
		{PublicKey: p.User, IsWritable: true, IsSigner: true},
		{PublicKey: solana.SystemProgramID, IsWritable: false, IsSigner: false},
		{PublicKey: creatorVault, IsWritable: true, IsSigner: false},
		{PublicKey: p.BaseTokenProgram, IsWritable: false, IsSigner: false},
		{PublicKey: eventAuth, IsWritable: false, IsSigner: false},
		{PublicKey: program, IsWritable: false, IsSigner: false},
		{PublicKey: feeConfig, IsWritable: false, IsSigner: false},
		{PublicKey: FeeProgramID(), IsWritable: false, IsSigner: false},
	}
	if p.CashbackEnabled {
		uva, err := UserVolumeAccumulatorPDA(p.User)
		if err != nil {
			return nil, err
		}
		sellAccounts = append(sellAccounts, &solana.AccountMeta{
			PublicKey: uva, IsWritable: true, IsSigner: false,
		})
	}
	sellAccounts = append(sellAccounts, &solana.AccountMeta{
		PublicKey: bcV2, IsWritable: false, IsSigner: false,
	})
	sellAccounts = append(sellAccounts, &solana.AccountMeta{
		PublicKey: buybackFeeRecipient, IsWritable: true, IsSigner: false,
	})
	ixs = append(ixs, solana.NewInstruction(program, sellAccounts, data))

	return ixs, nil
}

func bondingCurvePDAFromProgram(program, mint solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("bonding-curve"), mint.Bytes()},
		program,
	)
	return pda, err
}
