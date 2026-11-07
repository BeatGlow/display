package main

func init() {
	registerOfflineDriver("st7789", offlineDriverConfig{
		frameDataExtra:  8,
		contrast:        offlineContrastConfig{none: true},
		brightnessScale: func(uint8) uint8 { return 0xff },
	})
}
