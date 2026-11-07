package pixel

import "testing"

func TestFillPatternMatchesScalar(t *testing.T) {
	patterns := []uint64{0, 0x0123456789abcdef, ^uint64(0)}
	for _, size := range []int{0, 1, 7, 15, 16, 17, 31, 32, 33, 257} {
		for _, pattern := range patterns {
			got := make([]byte, size)
			want := make([]byte, size)
			fillPattern(got, pattern)
			fillPatternScalar(want, pattern)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("size %d pattern %#x: byte %d = %#x, want %#x", size, pattern, i, got[i], want[i])
				}
			}
		}
	}
}

func BenchmarkFill400x1280(b *testing.B) {
	for _, tc := range []struct {
		name       string
		size       int
		pattern    uint64
		scalarFill func([]byte)
	}{
		{
			name:    "Gray4",
			size:    400 * 1280 / 2,
			pattern: 0x7777777777777777,
			scalarFill: func(dst []byte) {
				for i := range dst {
					dst[i] = 0x77
				}
			},
		},
		{
			name:    "CRGB16",
			size:    400 * 1280 * 2,
			pattern: 0x3412341234123412,
			scalarFill: func(dst []byte) {
				bytes := [2]byte{0x12, 0x34}
				for i := 0; i < len(dst); i += 2 {
					copy(dst[i:], bytes[:])
				}
			},
		},
	} {
		b.Run(tc.name+"/SIMD", func(b *testing.B) {
			dst := make([]byte, tc.size)
			b.SetBytes(int64(len(dst)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				fillPattern(dst, tc.pattern)
			}
		})
		b.Run(tc.name+"/Scalar", func(b *testing.B) {
			dst := make([]byte, tc.size)
			b.SetBytes(int64(len(dst)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.scalarFill(dst)
			}
		})
	}
}
