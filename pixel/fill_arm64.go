//go:build arm64

package pixel

func fillSIMD(dst []byte, pattern uint64) int {
	return fillSIMDARM64(dst, pattern)
}

func fillSIMDARM64(dst []byte, pattern uint64) int
