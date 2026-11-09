//go:build !arm64 && !amd64

package buffer

func rotateXRGB90(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	rotateXRGB90Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB270(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	rotateXRGB270Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB180(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	rotateXRGB180Go(dst, dstStride, src, srcStride, width, height)
}

func rotateRGBA180([]byte, int, []byte, int, int, int) bool { return false }
func rotateRGBA90([]byte, int, []byte, int, int, int) bool  { return false }
func rotateRGBA270([]byte, int, []byte, int, int, int) bool { return false }
