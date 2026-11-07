//go:build !arm64

package buffer

func copyRGBAChunk(dst, src []byte) int { return 0 }
