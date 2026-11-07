package pixel

// fillRepeatedByte fills dst with value using the fastest implementation for
// the current architecture.
func fillRepeatedByte(dst []byte, value byte) {
	pattern := uint64(value) * 0x0101010101010101
	fillPattern(dst, pattern)
}

// fillRepeatedWord fills dst with the two-byte sequence b0, b1.
func fillRepeatedWord(dst []byte, b0, b1 byte) {
	pattern := uint64(b0) | uint64(b1)<<8
	pattern |= pattern << 16
	pattern |= pattern << 32
	fillPattern(dst, pattern)
}

func fillPattern(dst []byte, pattern uint64) {
	n := fillSIMD(dst, pattern)
	fillPatternScalar(dst[n:], pattern)
}

func fillPatternScalar(dst []byte, pattern uint64) {
	for i := range dst {
		dst[i] = byte(pattern >> (uint(i&7) * 8))
	}
}
