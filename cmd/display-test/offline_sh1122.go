package main

func init() {
	registerOfflineDriver("sh1122", offlineDriverConfig{contrast: offlineCommandContrast(0x81, singleByte)})
}
