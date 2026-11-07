package main

import "image"

func init() {
	registerOfflineDriver("gp1294", offlineDriverConfig{
		initialization: [][]byte{
			{0xAA}, {0x78, 0x08}, {0xCC, 0x01, 0x1F, 0x00, 0xFF, 0x2F, 0x00, 0x20},
			{0xA0, 0x28, 0x00}, {0xC0, 0x00, 0x00}, {0x08, 0x00}, {0x80, 0x00},
			{0xF0, 0x00, 0x00, 0x7F}, {0x6D},
		},
		reverseInitBits: true, reverseFrameBits: true, monoLayout: monoColumnMajor, frameInCommand: true,
		frameCommandPrefix: func(frame image.Image) []byte {
			return []byte{reverseBits(0xf0), 0, 0, reverseBits(byte(frame.Bounds().Dy() - 1))}
		},
		contrast: reverseContrast(0xa0, func(level uint8) []byte {
			v := uint16(level) << 2
			return []byte{byte(v), byte(v >> 8)}
		}),
		brightnessScale: func(level uint8) uint8 { return uint8(uint16(level) * 4 * 255 / 1023) },
	})
}
