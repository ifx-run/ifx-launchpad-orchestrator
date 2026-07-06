package bridge

// AmountInDataOffset is the CPI data byte offset for swap amount_in patching.
func (t PoolType) AmountInDataOffset() uint16 {
	switch t {
	case PoolRaydiumAMMv4:
		return 1
	case PoolRaydiumCPMM:
		return 8
	default:
		return 8
	}
}
