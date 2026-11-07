#include "textflag.h"

// copyXRGBRowARM64 converts RGBA pixels to little-endian XRGB8888 using NEON.
// The buffers contain a multiple of 16 pixels.
TEXT ·copyXRGBRowARM64(SB), NOSPLIT, $0-48
	MOVD dst_base+0(FP), R0
	MOVD dst_len+8(FP), R1
	MOVD src_base+24(FP), R2

loop:
	CMP $64, R1
	BLT done
	VLD4.P 64(R2), [V4.B16, V5.B16, V6.B16, V7.B16]
	VMOV V6.B16, V0.B16
	VMOV V5.B16, V1.B16
	VMOV V4.B16, V2.B16
	VEOR V3.B16, V3.B16, V3.B16
	VST4.P [V0.B16, V1.B16, V2.B16, V3.B16], 64(R0)
	SUB $64, R1
	B loop

done:
	RET
