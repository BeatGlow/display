//go:build !arm64 && !amd64

package pixel

func copyYU12LumaChunk(dst, src []byte) int { return 0 }
