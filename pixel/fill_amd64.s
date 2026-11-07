#include "textflag.h"

// fillSIMD repeats pattern across dst in 16-byte SSE2 stores.
TEXT ·fillSIMD(SB), NOSPLIT, $0-40
	MOVQ dst_base+0(FP), AX
	MOVQ dst_len+8(FP), BX
	MOVQ pattern+24(FP), CX
	MOVQ CX, X0
	PUNPCKLQDQ X0, X0
	XORQ DX, DX

loop:
	CMPQ BX, $16
	JL done
	MOVOU X0, (AX)
	ADDQ $16, AX
	SUBQ $16, BX
	ADDQ $16, DX
	JMP loop

done:
	MOVQ DX, ret+32(FP)
	RET
