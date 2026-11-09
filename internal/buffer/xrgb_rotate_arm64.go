//go:build arm64

package buffer

func rotateXRGB90(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 == 0 && height&3 == 0 {
		rotateXRGB90ARM64(dst, dstStride, src, srcStride, width, height)
		return
	}
	rotateXRGB90Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB270(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 == 0 && height&3 == 0 {
		rotateXRGB270ARM64(dst, dstStride, src, srcStride, width, height)
		return
	}
	rotateXRGB270Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB90ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height int)
func rotateXRGB270ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height int)
