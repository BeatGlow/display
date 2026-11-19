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

// Pack four source rows, transpose their pixels, and store source columns
// from right to left into successive scanout rows. V12 holds the byte indices.
#define STORE_RGBA90_TILE(A, B, C, D, dst) \
	VTBL V12.B16, [A.B16], A.B16; \
	VTBL V12.B16, [B.B16], B.B16; \
	VTBL V12.B16, [C.B16], C.B16; \
	VTBL V12.B16, [D.B16], D.B16; \
	VTRN1 B.S4, A.S4, V16.S4; \
	VTRN2 B.S4, A.S4, V17.S4; \
	VTRN1 D.S4, C.S4, V18.S4; \
	VTRN2 D.S4, C.S4, V19.S4; \
	VTRN2 V19.D2, V17.D2, V20.D2; \
	VTRN2 V18.D2, V16.D2, V21.D2; \
	VTRN1 V19.D2, V17.D2, V22.D2; \
	VTRN1 V18.D2, V16.D2, V23.D2; \
	MOVD dst, R13; \
	VST1.P [V20.B16], (R13)(R1); \
	VST1.P [V21.B16], (R13)(R1); \
	VST1.P [V22.B16], (R13)(R1); \
	VST1 [V23.B16], (R13)

// rotateRGBA90ARM64 packs RGBA channels while transposing into clockwise XRGB order.
TEXT ·rotateRGBA90ARM64(SB), NOSPLIT, $0-96
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
	MOVD $rgbaToXRGB<>(SB), R14
	VLD1 (R14), [V12.B16]

	CBZ R4, doneRGBA90
yloopWideRGBA90:
	ADD $8, R6, R7
	CMP R15, R7
	BGT tailRGBA90
	// Start at source column height-8-y and destination row y.
	SUB R6, R5, R8
	SUB $8, R8, R8
	ADD R8<<2, R2, R8
	MADD R1, R0, R6, R9
	ADD R1<<2, R9, R14
	MOVD R4, R7

xloopWideRGBA90:
	VLD1.P (R8)(R3), [V0.S4, V1.S4]
	VLD1.P (R8)(R3), [V2.S4, V3.S4]
	VLD1.P (R8)(R3), [V4.S4, V5.S4]
	VLD1.P (R8)(R3), [V6.S4, V7.S4]
	STORE_RGBA90_TILE(V1, V3, V5, V7, R9)
	STORE_RGBA90_TILE(V0, V2, V4, V6, R14)
	ADD $16, R14
	ADD $16, R9
	SUBS $4, R7
	BGT xloopWideRGBA90
	ADD $8, R6
	B yloopWideRGBA90

// Worker bands are four-row aligned and may end with one 4x4 tile row.
tailRGBA90:
	CMP R15, R6
	BGE doneRGBA90
	// Start at source column height-4-y and destination row y.
	SUB R6, R5, R8
	SUB $4, R8, R8
	ADD R8<<2, R2, R8
	MADD R1, R0, R6, R9
	MOVD R4, R7

xloopTailRGBA90:
	VLD1.P (R8)(R3), [V0.S4]
	VLD1.P (R8)(R3), [V1.S4]
	VLD1.P (R8)(R3), [V2.S4]
	VLD1.P (R8)(R3), [V3.S4]
	STORE_RGBA90_TILE(V0, V1, V2, V3, R9)
	ADD $16, R9
	SUBS $4, R7
	BGT xloopTailRGBA90

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

// Select B,G,R from each RGBA pixel. Out-of-range indices produce zero.
DATA rgbaToXRGB<>+0(SB)/8, $0xff040506ff000102
DATA rgbaToXRGB<>+8(SB)/8, $0xff0c0d0eff08090a
GLOBL rgbaToXRGB<>(SB), RODATA|NOPTR, $16
