package main

func init() {
	registerOfflineDriver("st7735", offlineDriverConfig{frameDataExtra: 8, contrast: offlineContrastConfig{pwm: true}})
}
