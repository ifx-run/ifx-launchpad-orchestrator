package solana

import (
	"encoding/binary"

	"github.com/gagliardetto/solana-go"
)

// SPL Token UnwrapLamports instruction discriminator.
const tokenIxUnwrapLamports byte = 45

// UnwrapLamportsInstruction transfers lamports from a native (WSOL) token account to destination.
// amount nil unwraps the entire synced balance; non-nil unwraps a partial amount without closing the ATA.
func UnwrapLamportsInstruction(
	source, destination, owner, tokenProgram solana.PublicKey,
	amount *uint64,
) solana.Instruction {
	data := []byte{tokenIxUnwrapLamports}
	if amount == nil {
		data = append(data, 0)
	} else {
		data = append(data, 1)
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], *amount)
		data = append(data, buf[:]...)
	}
	return solana.NewInstruction(
		tokenProgram,
		solana.AccountMetaSlice{
			{PublicKey: source, IsWritable: true, IsSigner: false},
			{PublicKey: destination, IsWritable: true, IsSigner: false},
			{PublicKey: owner, IsWritable: false, IsSigner: true},
		},
		data,
	)
}
