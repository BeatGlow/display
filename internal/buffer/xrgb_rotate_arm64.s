#include "textflag.h"

// rotateXRGB90ARM64 transposes 4x4 XRGB8888 tiles into clockwise scanout order.
TEXT ·rotateXRGB90ARM64(SB), NOSPLIT, $0-96
	MOVD dst_base+0(FP), R0
	MOVD dstStride+24(FP), R1
	MOVD src_base+32(FP), R2
	MOVD srcStride+56(FP), R3
	MOVD width+64(FP), R4
	MOVD height+72(FP), R5
	AND $-4, R4, R4
	AND $-4, R5, R5
	MOVD startY+80(FP), R6
	MOVD endY+88(FP), R15

yloop90:
	CMP R15, R6
	BGE done90
	MOVD $0, R7

xloop90:
	CMP R4, R7
	BGE nexty90
	ADD $3, R7, R9
	MUL R9, R3, R9
	ADD R2, R9, R8
	SUB R6, R5, R10
	SUB $4, R10, R10
	LSL $2, R10, R10
	ADD R10, R8, R8
	MOVD R8, R12
	VLD1 (R12), [V0.B16]
	SUB R3, R8, R12
	VLD1 (R12), [V1.B16]
	SUB R3, R12, R12
	VLD1 (R12), [V2.B16]
	SUB R3, R12, R12
	VLD1 (R12), [V3.B16]
	VTRN1 V0.S4, V1.S4, V4.S4
	VTRN2 V0.S4, V1.S4, V5.S4
	VTRN1 V2.S4, V3.S4, V6.S4
	VTRN2 V2.S4, V3.S4, V7.S4
	VTRN1 V4.D2, V6.D2, V8.D2
	VTRN2 V4.D2, V6.D2, V9.D2
	VTRN1 V5.D2, V7.D2, V10.D2
	VTRN2 V5.D2, V7.D2, V11.D2
	MUL R6, R1, R13
	ADD R0, R13, R13
	MOVD R7, R12
	LSL $2, R12, R12
	ADD R12, R13, R13
	VST1 [V11.B16], (R13)
	ADD R1, R13, R13
	VST1 [V9.B16], (R13)
	ADD R1, R13, R13
	VST1 [V10.B16], (R13)
	ADD R1, R13, R13
	VST1 [V8.B16], (R13)
	ADD $4, R7, R7
	B xloop90

nexty90:
	ADD $4, R6, R6
	B yloop90

done90:
	RET

// rotateXRGB270ARM64 transposes 4x4 XRGB8888 tiles into counterclockwise scanout order.
TEXT ·rotateXRGB270ARM64(SB), NOSPLIT, $0-96
	MOVD dst_base+0(FP), R0
	MOVD dstStride+24(FP), R1
	MOVD src_base+32(FP), R2
	MOVD srcStride+56(FP), R3
	MOVD width+64(FP), R4
	MOVD height+72(FP), R5
	AND $-4, R4, R4
	AND $-4, R5, R5
	MOVD startY+80(FP), R6
	MOVD endY+88(FP), R15

yloop270:
	CMP R15, R6
	BGE done270
	MOVD $0, R7

xloop270:
	CMP R4, R7
	BGE nexty270
	SUB $4, R4, R9
	SUB R7, R9, R9
	MUL R9, R3, R9
	ADD R2, R9, R8
	MOVD R6, R10
	LSL $2, R10, R10
	ADD R10, R8, R8
	MOVD R8, R12
	VLD1 (R12), [V0.B16]
	ADD R3, R8, R12
	VLD1 (R12), [V1.B16]
	ADD R3, R12, R12
	VLD1 (R12), [V2.B16]
	ADD R3, R12, R12
	VLD1 (R12), [V3.B16]
	VTRN1 V0.S4, V1.S4, V4.S4
	VTRN2 V0.S4, V1.S4, V5.S4
	VTRN1 V2.S4, V3.S4, V6.S4
	VTRN2 V2.S4, V3.S4, V7.S4
	VTRN1 V4.D2, V6.D2, V8.D2
	VTRN2 V4.D2, V6.D2, V9.D2
	VTRN1 V5.D2, V7.D2, V10.D2
	VTRN2 V5.D2, V7.D2, V11.D2
	MUL R6, R1, R13
	ADD R0, R13, R13
	MOVD R7, R12
	LSL $2, R12, R12
	ADD R12, R13, R13
	VST1 [V8.B16], (R13)
	ADD R1, R13, R13
	VST1 [V10.B16], (R13)
	ADD R1, R13, R13
	VST1 [V9.B16], (R13)
	ADD R1, R13, R13
	VST1 [V11.B16], (R13)
	ADD $4, R7, R7
	B xloop270

nexty270:
	ADD $4, R6, R6
	B yloop270

done270:
	RET

// rotateXRGB180ARM64 reverses XRGB pixels within each row and reverses row order.
TEXT ·rotateXRGB180ARM64(SB), NOSPLIT, $0-96
	MOVD dst_base+0(FP), R0
	MOVD dstStride+24(FP), R1
	MOVD src_base+32(FP), R2
	MOVD srcStride+56(FP), R3
	MOVD width+64(FP), R4
	MOVD height+72(FP), R5
	MOVD startY+80(FP), R6
	MOVD endY+88(FP), R15

yloop180:
	CMP R15, R6
	BGE done180
	SUB $1, R5, R8
	SUB R6, R8, R8
	MUL R3, R8, R8
	ADD R2, R8, R8
	MUL R1, R6, R9
	ADD R0, R9, R9
	MOVD $0, R7

xloop180:
	CMP R4, R7
	BGE nexty180
	SUB $4, R4, R10
	SUB R7, R10, R10
	LSL $2, R10, R10
	ADD R8, R10, R10
	VLD1 (R10), [V0.B16]
	VREV64 V0.B16, V1.B16
	VEXT $8, V1.B16, V1.B16, V2.B16
	VREV32 V2.B16, V2.B16
	MOVD R7, R11
	LSL $2, R11, R11
	ADD R9, R11, R11
	VST1 [V2.B16], (R11)
	ADD $4, R7, R7
	B xloop180

nexty180:
	ADD $1, R6, R6
	B yloop180

done180:
	RET

// rotateRGBA180ARM64 packs RGBA channels while reversing the image.
TEXT ·rotateRGBA180ARM64(SB), NOSPLIT, $0-96
	MOVD dst_base+0(FP), R0
	MOVD dstStride+24(FP), R1
	MOVD src_base+32(FP), R2
	MOVD srcStride+56(FP), R3
	MOVD width+64(FP), R4
	MOVD height+72(FP), R5
	MOVD $0x00ffffff, R14
	VDUP R14, V12.S4
	MOVD startY+80(FP), R6
	MOVD endY+88(FP), R15

yloopRGBA180:
	CMP R15, R6
	BGE doneRGBA180
	SUB $1, R5, R8
	SUB R6, R8, R8
	MUL R3, R8, R8
	ADD R2, R8, R8
	MUL R1, R6, R9
	ADD R0, R9, R9
	MOVD $0, R7

xloopRGBA180:
	CMP R4, R7
	BGE nextyRGBA180
	SUB $4, R4, R10
	SUB R7, R10, R10
	LSL $2, R10, R10
	ADD R8, R10, R10
	VLD1 (R10), [V0.B16]
	VREV32 V0.B16, V0.B16
	VEXT $1, V0.B16, V0.B16, V0.B16
	VAND V12.B16, V0.B16, V0.B16
	VREV64 V0.B16, V1.B16
	VEXT $8, V1.B16, V1.B16, V2.B16
	VREV32 V2.B16, V2.B16
	MOVD R7, R11
	LSL $2, R11, R11
	ADD R9, R11, R11
	VST1 [V2.B16], (R11)
	ADD $4, R7, R7
	B xloopRGBA180

nextyRGBA180:
	ADD $1, R6, R6
	B yloopRGBA180

doneRGBA180:
	RET

// Transpose four source rows. Advance each row pointer to the next group
// of four rows, allowing independent loads at the next iteration.
#define TRANSPOSE_RGBA90(A, B, C, D) \
	VLD1.P (R8)(R11), [V0.S4]; \
	VLD1.P (R9)(R11), [V1.S4]; \
	VLD1.P (R10)(R11), [V2.S4]; \
	VLD1.P (R12)(R11), [V3.S4]; \
	VTBL V12.B16, [V0.B16], V0.B16; \
	VTBL V12.B16, [V1.B16], V1.B16; \
	VTBL V12.B16, [V2.B16], V2.B16; \
	VTBL V12.B16, [V3.B16], V3.B16; \
	VTRN1 V1.S4, V0.S4, V4.S4; \
	VTRN2 V1.S4, V0.S4, V5.S4; \
	VTRN1 V3.S4, V2.S4, V6.S4; \
	VTRN2 V3.S4, V2.S4, V7.S4; \
	VTRN2 V7.D2, V5.D2, A.D2; \
	VTRN2 V6.D2, V4.D2, B.D2; \
	VTRN1 V7.D2, V5.D2, C.D2; \
	VTRN1 V6.D2, V4.D2, D.D2

// rotateRGBA90ARM64 rotates RGBA into clockwise XRGB scanout order.
TEXT ·rotateRGBA90ARM64(SB), NOSPLIT, $0-96
	MOVD dst_base+0(FP), R0
	MOVD dstStride+24(FP), R1
	MOVD src_base+32(FP), R2
	MOVD srcStride+56(FP), R3
	MOVD width+64(FP), R4
	MOVD height+72(FP), R5
	AND $-4, R4, R4
	AND $-4, R5, R5
	MOVD endY+88(FP), R15
	MOVD $rgbaToXRGB<>(SB), R14
	VLD1 (R14), [V12.B16]

	CBZ R4, doneRGBA90
	LSL $2, R3, R11
	MOVD $0, R19
// Sixteen source rows produce one 64-byte store per scanout row.
stripRGBA90:
	SUB R19, R4, R20
	CMP $16, R20
	BLE stripWidthRGBA90
	MOVD $16, R20
stripWidthRGBA90:
	// R21 walks left by four source pixels as R22 advances four scanout rows.
	MOVD startY+80(FP), R6
	SUB R6, R5, R21
	SUB $4, R21
	ADD R21<<2, R2, R21
	MADD R3, R21, R19, R21
	MADD R1, R0, R6, R22
	ADD R19<<2, R22, R22
yloopRGBA90:
	CMP R15, R6
	BGE nextStripRGBA90
	MOVD R21, R8
	ADD R3, R8, R9
	ADD R3, R9, R10
	ADD R3, R10, R12
	MOVD R22, R13
	ADD R1, R13, R14
	ADD R1, R14, R16
	ADD R1, R16, R17
	MOVD R20, R7
	CMP $16, R7
	BLT tailRGBA90
	// Accumulate 16 output pixels per row before touching the DRM mapping.
	TRANSPOSE_RGBA90(V16, V20, V24, V28)
	TRANSPOSE_RGBA90(V17, V21, V25, V29)
	TRANSPOSE_RGBA90(V18, V22, V26, V30)
	TRANSPOSE_RGBA90(V19, V23, V27, V31)
	VST1.P [V16.B16, V17.B16, V18.B16, V19.B16], 64(R13)
	VST1.P [V20.B16, V21.B16, V22.B16, V23.B16], 64(R14)
	VST1.P [V24.B16, V25.B16, V26.B16, V27.B16], 64(R16)
	VST1.P [V28.B16, V29.B16, V30.B16, V31.B16], 64(R17)
	B nextRGBA90
tailRGBA90:
	CBZ R7, nextRGBA90
	TRANSPOSE_RGBA90(V16, V20, V24, V28)
	VST1.P [V16.B16], 16(R13)
	VST1.P [V20.B16], 16(R14)
	VST1.P [V24.B16], 16(R16)
	VST1.P [V28.B16], 16(R17)
	SUB $4, R7
	B tailRGBA90
nextRGBA90:
	SUB $16, R21
	ADD R1<<2, R22, R22
	ADD $4, R6
	B yloopRGBA90
nextStripRGBA90:
	ADD R20, R19
	CMP R4, R19
	BLT stripRGBA90
doneRGBA90:
	RET

// rotateRGBA270ARM64 packs RGBA channels while transposing into counterclockwise XRGB order.
TEXT ·rotateRGBA270ARM64(SB), NOSPLIT, $0-96
	MOVD dst_base+0(FP), R0
	MOVD dstStride+24(FP), R1
	MOVD src_base+32(FP), R2
	MOVD srcStride+56(FP), R3
	MOVD width+64(FP), R4
	MOVD height+72(FP), R5
	AND $-4, R4, R4
	AND $-4, R5, R5
	MOVD startY+80(FP), R6
	MOVD endY+88(FP), R15
	MOVD $0x00ffffff, R14
	VDUP R14, V12.S4

yloopRGBA270:
	CMP R15, R6
	BGE doneRGBA270
	MOVD $0, R7

xloopRGBA270:
	CMP R4, R7
	BGE nextyRGBA270
	SUB $4, R4, R9
	SUB R7, R9, R9
	MUL R9, R3, R9
	ADD R2, R9, R8
	MOVD R6, R10
	LSL $2, R10, R10
	ADD R10, R8, R8
	MOVD R8, R12
	VLD1 (R12), [V0.B16]
	ADD R3, R8, R12
	VLD1 (R12), [V1.B16]
	ADD R3, R12, R12
	VLD1 (R12), [V2.B16]
	ADD R3, R12, R12
	VLD1 (R12), [V3.B16]
	VREV32 V0.B16, V0.B16
	VEXT $1, V0.B16, V0.B16, V0.B16
	VAND V12.B16, V0.B16, V0.B16
	VREV32 V1.B16, V1.B16
	VEXT $1, V1.B16, V1.B16, V1.B16
	VAND V12.B16, V1.B16, V1.B16
	VREV32 V2.B16, V2.B16
	VEXT $1, V2.B16, V2.B16, V2.B16
	VAND V12.B16, V2.B16, V2.B16
	VREV32 V3.B16, V3.B16
	VEXT $1, V3.B16, V3.B16, V3.B16
	VAND V12.B16, V3.B16, V3.B16
	VTRN1 V0.S4, V1.S4, V4.S4
	VTRN2 V0.S4, V1.S4, V5.S4
	VTRN1 V2.S4, V3.S4, V6.S4
	VTRN2 V2.S4, V3.S4, V7.S4
	VTRN1 V4.D2, V6.D2, V8.D2
	VTRN2 V4.D2, V6.D2, V9.D2
	VTRN1 V5.D2, V7.D2, V10.D2
	VTRN2 V5.D2, V7.D2, V11.D2
	MUL R6, R1, R13
	ADD R0, R13, R13
	MOVD R7, R12
	LSL $2, R12, R12
	ADD R12, R13, R13
	VST1 [V8.B16], (R13)
	ADD R1, R13, R13
	VST1 [V10.B16], (R13)
	ADD R1, R13, R13
	VST1 [V9.B16], (R13)
	ADD R1, R13, R13
	VST1 [V11.B16], (R13)
	ADD $4, R7, R7
	B xloopRGBA270

nextyRGBA270:
	ADD $4, R6, R6
	B yloopRGBA270

doneRGBA270:
	RET

// Shuffle indices in little-endian memory order, grouped by output pixel:
//   02 01 00 ff | 06 05 04 ff | 0a 09 08 ff | 0e 0d 0c ff
// Each group selects B, G, R. For X, VTBL returns zero because index 0xff
// is outside the 16-byte source table (valid indices 0x00 through 0x0f).
DATA rgbaToXRGB<>+0(SB)/8, $0xff040506ff000102
DATA rgbaToXRGB<>+8(SB)/8, $0xff0c0d0eff08090a
GLOBL rgbaToXRGB<>(SB), RODATA|NOPTR, $16
