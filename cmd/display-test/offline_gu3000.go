package main

import "image"

func init() {
	registerOfflineDriver("gu3000", offlineDriverConfig{
		frameDataPrefix: func(frame image.Image) []byte {
			size := uint16(frame.Bounds().Dx() * frame.Bounds().Dy() / 8)
			return []byte{0x02, 0x44, 0xff, 0x46, 0x00, 0x00, byte(size), byte(size >> 8)}
		},
		contrast: offlineDataContrast(func(level uint8) []byte { return []byte{0x02, 0x44, 0xff, 0x58, level} }),
	})
}
