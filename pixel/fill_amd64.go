//go:build amd64

package pixel

func fillSIMD(dst []byte, pattern uint64) int {
	return fillSIMDAMD64(dst, pattern)
}

func fillSIMDAMD64(dst []byte, pattern uint64) int
