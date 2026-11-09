//go:build amd64

package buffer

func rotateXRGB90(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 != 0 || height&3 != 0 {
		rotateXRGB90Go(dst, dstStride, src, srcStride, width, height)
		return
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 64, rotateXRGB90AMD64)
}

func rotateXRGB270(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 != 0 || height&3 != 0 {
		rotateXRGB270Go(dst, dstStride, src, srcStride, width, height)
		return
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 64, rotateXRGB270AMD64)
}

func rotateXRGB180(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 != 0 {
		rotateXRGB180Go(dst, dstStride, src, srcStride, width, height)
		return
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 64, rotateXRGB180AMD64)
}

func rotateRGBA180(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 {
		return false
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 64, rotateRGBA180AMD64)
	return true
}

func rotateRGBA90(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 || height&3 != 0 {
		return false
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 64, rotateRGBA90AMD64)
	return true
}

func rotateRGBA270(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 || height&3 != 0 {
		return false
	}
	parallelRotate(dst, dstStride, src, srcStride, width, height, 64, rotateRGBA270AMD64)
	return true
}

func rotateXRGB90AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateXRGB270AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateXRGB180AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA90AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA270AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA180AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
