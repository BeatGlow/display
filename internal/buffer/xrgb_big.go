//go:build mips || mips64 || ppc64 || s390x

package buffer

func putXRGB(dst []byte, red, green, blue uint8) {
	dst[0], dst[1], dst[2], dst[3] = 0, red, green, blue
}
