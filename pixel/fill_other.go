//go:build !amd64 && !arm64

package pixel

func fillSIMD(dst []byte, pattern uint64) int {
	return 0
}
