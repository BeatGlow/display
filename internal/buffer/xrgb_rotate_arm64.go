//go:build arm64

package buffer

func rotateXRGB90(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 == 0 && height&3 == 0 {
		parallelRotate(dst, dstStride, src, srcStride, width, height, 4, rotateXRGB90ARM64)
		return
	}
	rotateXRGB90Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB270(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 == 0 && height&3 == 0 {
		parallelRotate(dst, dstStride, src, srcStride, width, height, 4, rotateXRGB270ARM64)
		return
	}
	rotateXRGB270Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB180(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 == 0 {
		parallelRotate(dst, dstStride, src, srcStride, width, height, 4, rotateXRGB180ARM64)
		return
	}
	rotateXRGB180Go(dst, dstStride, src, srcStride, width, height)
}

func rotateRGBA180(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 {
		return false
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 4, rotateRGBA180ARM64)
	return true
}

func rotateRGBA90(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 || height&3 != 0 {
		return false
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 4, rotateRGBA90ARM64)
	return true
}

func rotateRGBA270(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 || height&3 != 0 {
		return false
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 4, rotateRGBA270ARM64)
	return true
}

func rotateXRGB90ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateXRGB270ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateXRGB180ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA180ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA90ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA270ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
