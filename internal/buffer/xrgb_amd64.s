#include "textflag.h"

// copyXRGBRowAMD64 converts RGBA pixels to little-endian XRGB8888 using SSE2.
// The buffers contain a multiple of 16 pixels.
TEXT ·copyXRGBRowAMD64(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), AX
	MOVQ dst_len+8(FP), BX
	MOVQ src_base+24(FP), CX
	MOVOU ·xrgbBlueMask<>(SB), X1
	MOVOU ·xrgbGreenMask<>(SB), X2
	MOVOU ·xrgbRedMask<>(SB), X3

loop:
	CMPQ BX, $64
	JL done

	MOVOU 0(CX), X0
	MOVOU X0, X4
	MOVOU X0, X5
	PAND X1, X0
	PSRLL $16, X0
	PAND X2, X4
	PAND X3, X5
	PSLLL $16, X5
	POR X4, X0
	POR X5, X0
	MOVOU X0, 0(AX)

	MOVOU 16(CX), X0
	MOVOU X0, X4
	MOVOU X0, X5
	PAND X1, X0
	PSRLL $16, X0
	PAND X2, X4
	PAND X3, X5
	PSLLL $16, X5
	POR X4, X0
	POR X5, X0
	MOVOU X0, 16(AX)

	MOVOU 32(CX), X0
	MOVOU X0, X4
	MOVOU X0, X5
	PAND X1, X0
	PSRLL $16, X0
	PAND X2, X4
	PAND X3, X5
	PSLLL $16, X5
	POR X4, X0
	POR X5, X0
	MOVOU X0, 32(AX)

	MOVOU 48(CX), X0
	MOVOU X0, X4
	MOVOU X0, X5
	PAND X1, X0
	PSRLL $16, X0
	PAND X2, X4
	PAND X3, X5
	PSLLL $16, X5
	POR X4, X0
	POR X5, X0
	MOVOU X0, 48(AX)

	ADDQ $64, CX
	ADDQ $64, AX
	SUBQ $64, BX
	JMP loop

done:
	RET

DATA ·xrgbBlueMask<>+0(SB)/8, $0x00ff000000ff0000
DATA ·xrgbBlueMask<>+8(SB)/8, $0x00ff000000ff0000
GLOBL ·xrgbBlueMask<>(SB), RODATA|NOPTR, $16

DATA ·xrgbGreenMask<>+0(SB)/8, $0x0000ff000000ff00
DATA ·xrgbGreenMask<>+8(SB)/8, $0x0000ff000000ff00
GLOBL ·xrgbGreenMask<>(SB), RODATA|NOPTR, $16

DATA ·xrgbRedMask<>+0(SB)/8, $0x000000ff000000ff
DATA ·xrgbRedMask<>+8(SB)/8, $0x000000ff000000ff
GLOBL ·xrgbRedMask<>(SB), RODATA|NOPTR, $16
