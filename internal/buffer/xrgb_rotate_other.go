//go:build !arm64

package buffer

func rotateXRGB90(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	rotateXRGB90Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB270(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	rotateXRGB270Go(dst, dstStride, src, srcStride, width, height)
}
