package main

import (
	"image"
)

func init() {
	registerOfflineDriver("gu256x64d7000", offlineDriverConfig{
		frameDataPrefix: func(frame image.Image) []byte {
			w, h := frame.Bounds().Dx(), frame.Bounds().Dy()
			return []byte{0x1f, 0x28, 0x66, 0x11, byte(w), byte(w >> 8), byte(h / 8), byte((h / 8) >> 8), 0x01}
		},
		contrast: offlineDataContrast(func(level uint8) []byte {
			brightness := byte((uint16(level)*7)/255 + 1)
			return []byte{0x1f, 0x58, brightness}
		}),
		brightnessScale: func(level uint8) uint8 {
			return uint8(((uint16(level) * 7 / 255) + 1) * 255 / 8)
		},
	})
}
