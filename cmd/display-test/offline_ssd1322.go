package main

func init() {
	registerOfflineDriver("ssd1322", offlineDriverConfig{contrast: offlineCommandContrast(0xc1, singleByte)})
}
