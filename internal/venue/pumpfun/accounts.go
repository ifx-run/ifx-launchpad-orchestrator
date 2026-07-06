package pumpfun

import (
	"encoding/binary"

	"github.com/gagliardetto/solana-go"
)

const (
	ProgramIDBase58    = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"
	FeeProgramIDBase58 = "pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ"
)

var (
	discBuyExactSolIn      = [8]byte{56, 252, 116, 8, 158, 223, 205, 95}
	discSell               = [8]byte{51, 230, 133, 164, 1, 127, 131, 173}
	discBuyExactQuoteInV2  = [8]byte{194, 171, 28, 70, 104, 77, 91, 47}
	discSellV2             = [8]byte{93, 246, 130, 60, 231, 233, 64, 178}
	feeConfigConst    = []byte{
		1, 86, 224, 246, 147, 102, 90, 207, 68, 219, 21, 104, 191, 23,
		91, 170, 81, 137, 203, 151, 245, 210, 255, 59, 101, 93, 43,
		182, 253, 109, 24, 176,
	}
	pumpFeeRecipients = []string{
		"62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV",
		"7VtfL8fvgNfhz17qKRMjzQEXgbdpnHHHQRh54R9jP2RJ",
		"7hTckgnGnLQR6sdH7YkqFTAA7VwTfYFaZ6EhEsU3saCX",
		"9rPYyANsfQZw3DnDmKE3YCQF5E8oD89UXoHn9JFEhJUz",
		"AVmoTthdrX6tKt4nDjco2D775W2YK3sDhxPcMmzUAmTY",
		"CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM",
		"FWsW1xNtWscwNmKv6wVsU1iTzRN6wmmk3MjxRP5tT7hz",
		"G5UZAVbAf46s7cKWoyKu8kYTip9DGTpbLZ2qa9Aq69dP",
	}
	pumpReservedFeeRecipients = []string{
		"GesfTA3X2arioaHp8bbKdjG9vJtskViWACZoYvxp4twS",
		"4budycTjhs9fD6xw62VBducVTNgMgJJ5BgtKq7mAZwn6",
		"8SBKzEQU4nLSzcwF4a74F2iaUDQyTfjGndn6qUWBnrpR",
		"4UQeTP1T39KZ9Sfxzo3WR5skgsaP6NZa87BAkuazLEKH",
		"8sNeir4QsLsJdYpc9RZacohhK1Y5FLU3nC5LXgYB4aa6",
		"Fh9HmeLNUMVCvejxCtCL2DbYaRyBFVJ5xrWkLnMH6fdk",
		"463MEnMeGyJekNZFQSTUABBEbLnvMTALbT6ZmsxAbAdq",
		"6AUH3WEHucYZyC61hqpqYUWVto5qA5hjHuNQ32GNnNxA",
	}
	pumpBuybackFeeRecipients = []string{
		"5YxQFdt3Tr9zJLvkFccqXVUwhdTWJQc1fFg2YPbxvxeD",
		"9M4giFFMxmFGXtc3feFzRai56WbBqehoSeRE5GK7gf7",
		"GXPFM2caqTtQYC2cJ5yJRi9VDkpsYZXzYdwYpGnLmtDL",
		"3BpXnfJaUTiwXnJNe7Ej1rcbzqTTQUvLShZaWazebsVR",
		"5cjcW9wExnJJiqgLjq7DEG75Pm6JBgE1hNv4B2vHXUW6",
		"EHAAiTxcdDwQ3U4bU6YcMsQGaekdzLS3B5SmYo46kJtL",
		"5eHhjP8JaYkz83CWwvGU2uMUXefd3AazWGx4gpcuEEYD",
		"A7hAgCzFw14fejgCp387JUJRMNyz4j89JKnhtKU8piqW",
	}
)

func ProgramID() solana.PublicKey {
	return solana.MustPublicKeyFromBase58(ProgramIDBase58)
}

func FeeProgramID() solana.PublicKey {
	return solana.MustPublicKeyFromBase58(FeeProgramIDBase58)
}

func EventAuthorityPDA() (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("__event_authority")}, ProgramID())
	return pda, err
}

func GlobalPDAFromProgram(program solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("global")}, program)
	return pda, err
}

func CreatorVaultPDA(creator solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("creator-vault"), creator.Bytes()},
		ProgramID(),
	)
	return pda, err
}

func GlobalVolumeAccumulatorPDA() (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("global_volume_accumulator")}, ProgramID())
	return pda, err
}

func UserVolumeAccumulatorPDA(user solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("user_volume_accumulator"), user.Bytes()},
		ProgramID(),
	)
	return pda, err
}

func FeeConfigPDA() (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("fee_config"), feeConfigConst},
		FeeProgramID(),
	)
	return pda, err
}

func BondingCurveV2PDA(mint solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("bonding-curve-v2"), mint.Bytes()},
		ProgramID(),
	)
	return pda, err
}

func SharingConfigPDA(baseMint solana.PublicKey) (solana.PublicKey, error) {
	pda, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("sharing-config"), baseMint.Bytes()},
		FeeProgramID(),
	)
	return pda, err
}

func PickFeeRecipient(mayhemMode bool) solana.PublicKey {
	list := pumpFeeRecipients
	if mayhemMode {
		list = pumpReservedFeeRecipients
	}
	return solana.MustPublicKeyFromBase58(list[0])
}

func PickBuybackFeeRecipient() solana.PublicKey {
	return solana.MustPublicKeyFromBase58(pumpBuybackFeeRecipients[0])
}

func putU64LE(buf []byte, v uint64) {
	binary.LittleEndian.PutUint64(buf, v)
}
