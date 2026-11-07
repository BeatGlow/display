package main

func init() {
	registerOfflineDriver("gp1247ai", offlineDriverConfig{
		initialization: [][]byte{
			{0xAA}, {0x78, 0x08}, {0xCC, 0x05, 0x00},
			{0xE0, 0xFC, 0x3E, 0x00, 0x20, 0x80, 0x80, 0x80},
			{0xB1, 0x20, 0x3F, 0x00, 0x01}, {0xA0, 0x02, 0x00}, {0x55},
			{0xC0, 0x00, 0x00}, {0x80, 0x80}, {0x6D}, {0x80, 0x80},
		},
		reverseInitBits: true, reverseFrameBits: true, monoLayout: monoPadded256,
		pageCommand: func(page int) []byte {
			return []byte{reverseBits(0xf0), 0, reverseBits(byte(page * 8)), reverseBits(0x07)}
		},
		contrast: reverseContrast(0xa0, func(level uint8) []byte {
			v := uint16(level) * 4
			return []byte{byte(v >> 8), byte(v)}
		}),
		brightnessScale: func(level uint8) uint8 { return uint8(uint16(level) * 4 * 255 / 1023) },
	})
}
