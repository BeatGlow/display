//go:build arm64

package buffer

func copyRGBAChunk(dst, src []byte) int {
	pixels := len(src) / 64 * 16
	if pixels != 0 {
		copyXRGBRowARM64(dst[:pixels*4], src[:pixels*4])
	}
	return pixels
}

func copyXRGBRowARM64(dst, src []byte)
