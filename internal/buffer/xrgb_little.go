//go:build 386 || amd64 || arm || arm64 || loong64 || mipsle || mips64le || ppc64le || riscv64 || wasm

package buffer

func putXRGB(dst []byte, red, green, blue uint8) {
	dst[0], dst[1], dst[2], dst[3] = blue, green, red, 0
}
