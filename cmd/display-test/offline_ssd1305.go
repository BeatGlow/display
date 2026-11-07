package main

func init() {
	registerOfflineDriver("ssd1305", offlineDriverConfig{contrast: offlineCommandContrast(0x81, singleByte)})
}
