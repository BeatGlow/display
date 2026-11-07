package main

func init() {
	registerOfflineDriver("ssd1306", offlineDriverConfig{contrast: offlineCommandContrast(0x81, singleByte)})
}
