package main

func init() {
	registerOfflineDriver("sh1106", offlineDriverConfig{contrast: offlineCommandContrast(0x81, singleByte)})
}
