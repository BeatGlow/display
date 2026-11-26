//go:build arm64 && !purego && !race

#include "textflag.h"

// Each major-axis step visits one clipped span along the minor axis.
// R0 is the first minor pixel on this scanline; R1 and R2 are its two strides.
TEXT ·probeStroke(SB), NOSPLIT|NOFRAME, $0-120
	MOVD pix_base+0(FP), R0
	MOVD majorStep+24(FP), R1
	MOVD minorStep+32(FP), R2
	MOVD count+40(FP), R3
	MOVD majorOffset+48(FP), R4
	FMOVD minorStart+56(FP), F3
	FMOVD dx+64(FP), F4
	FMOVD dy+72(FP), F5
	FMOVD minorMin+80(FP), F6
	FMOVD minorMax+88(FP), F7
	FMOVD reach+96(FP), F8
	MOVD quotients+112(FP), R12
	FCVTZSD F6, R5
	FMOVD $1.0, F9

	// Keep ink as four 16-bit channels for blending. Partial coverage uses
	// two double-precision pairs (RG, BA) to match the scalar pixel rounding.
	MOVWU ink+104(FP), R11
	VMOV R11, V0.S[0]
	VUXTL V0.B8, V0.H8
	VUXTL V0.H4, V1.S4
	VUXTL2 V1.S4, V2.D2
	VUXTL V1.S2, V1.D2
	VUCVTF V1.D2, V1.D2
	VUCVTF V2.D2, V2.D2
	MOVD $255, R10
	VDUP R10, V20.H4
	MOVD $127, R11
	VDUP R11, V21.H4
	MOVD $1, R11
	VDUP R11, V22.H4

scanline:
	// center = minorStart + dy * (majorOffset / dx).
	CBNZ R12, cachedFraction
	SCVTFD R4, F10
	FDIVD F4, F10, F10
center:
	FMADDD F5, F3, F10, F10
	FSUBD F8, F10, F11
	FRINTMD F11, F11
	FADDD F9, F11, F11
	FMAXD F6, F11, F11
	FADDD F8, F10, F12
	FRINTPD F12, F12
	FMIND F7, F12, F12
	FCMPD F12, F11
	BGE nextScanline
	FCVTZSD F11, R6
	FCVTZSD F12, R7
	SUB R5, R6, R8
	MADD R2, R0, R8, R8

pixel:
	SCVTFD R6, F13
	FSUBD F10, F13, F13
	FABSD F13, F13
	FSUBD F13, F8, F13
	FMOVD F0, F16
	FCMPD F9, F13
	BGE blend
	VMOV V13.D[0], V13.D[1]
	VFMUL V13.D2, V1.D2, V16.D2
	VFMUL V13.D2, V2.D2, V17.D2
	VFCVTZU V16.D2, V16.D2
	VFCVTZU V17.D2, V17.D2
	VUZP1 V17.S4, V16.S4, V16.S4
	VXTN V16.S4, V16.H4

blend:
	VMOV V16.H[3], R9
	CBZ R9, nextPixel
	CMP $255, R9
	BEQ storePixel

	// t = 255*src + (255-alpha)*dst. Accumulate all four channels in
	// u16 lanes; saturation also clamps non-premultiplied input.
	FMOVS (R8), F17
	VUXTL V17.B8, V17.H8
	SUB R9, R10, R9
	VDUP R9, V18.H4
	VMUL V20.H4, V16.H4, V16.H4
	VMUL V18.H4, V17.H4, V17.H4
	VUQADD V17.H4, V16.H4, V16.H4

	// Match (257*t+127)/65280 using shifts:
	// m = t + ((t+127)>>8) + 1; result = (m + (m>>8)) >> 8.
	VUQADD V21.H4, V16.H4, V17.H4
	VUSHR $8, V17.H4, V17.H4
	VUQADD V17.H4, V16.H4, V16.H4
	VUQADD V22.H4, V16.H4, V16.H4
	VUSHR $8, V16.H4, V17.H4
	VUQADD V17.H4, V16.H4, V16.H4
	VUSHR $8, V16.H4, V16.H4

storePixel:
	VXTN V16.H8, V16.B8
	FMOVS F16, (R8)

nextPixel:
	ADD R2, R8
	ADD $1, R6
	CMP R7, R6
	BLT pixel

nextScanline:
	ADD R1, R0
	ADD $1, R4
	SUBS $1, R3
	BNE scanline
	RET

cachedFraction:
	FMOVD.P 8(R12), F10
	B center
