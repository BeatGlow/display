//go:build noasm || (!amd64 && !arm64 && !(riscv64 && riscv64.rva23u64 && go1.26))

package jpegn

// rgbToYCbCrRow converts n pixels with a four byte stride to YCbCr planes.
func rgbToYCbCrRow(dstY, dstCb, dstCr, src []byte, n int) {
	rgbToYCbCrRowScalar(dstY, dstCb, dstCr, src, n)
}
