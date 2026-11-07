#include "textflag.h"

// fillSIMD repeats pattern across dst in 16-byte NEON stores.
TEXT ·fillSIMD(SB), NOSPLIT, $0-40
	MOVD dst_base+0(FP), R0
	MOVD dst_len+8(FP), R1
	MOVD pattern+24(FP), R2
	MOVD $0, R3
	VDUP R2, V0.D2

loop:
	CMP $16, R1
	BLT done
	VST1.P [V0.B16], 16(R0)
	SUB $16, R1
	ADD $16, R3
	B loop

done:
	MOVD R3, ret+32(FP)
	RET
