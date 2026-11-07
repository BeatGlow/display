//go:build amd64

package buffer

func copyRGBAChunk(dst, src []byte) int {
	pixels := len(src) / 64 * 16
	if pixels != 0 {
		copyXRGBRowAMD64(dst[:pixels*4], src[:pixels*4])
	}
	return pixels
}

func copyXRGBRowAMD64(dst, src []byte)
