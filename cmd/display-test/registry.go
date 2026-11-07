package main

import (
	"image"
	"strings"
)

type monoFrameLayout uint8

const (
	monoPageMajor monoFrameLayout = iota
	monoColumnMajor
	monoPadded256
)

type offlineDriverConfig struct {
	initialization     [][]byte
	reverseInitBits    bool
	reverseFrameBits   bool
	monoLayout         monoFrameLayout
	pageCommand        func(int) []byte
	frameInCommand     bool
	frameCommandPrefix func(image.Image) []byte
	frameDataPrefix    func(image.Image) []byte
	frameDataExtra     int
	contrast           offlineContrastConfig
	brightnessScale    func(uint8) uint8
}

type offlineContrastConfig struct {
	kind string
	data func(uint8) []byte
	pwm  bool
	none bool
}

var offlineDriverConfigs = map[string]offlineDriverConfig{}

func registerOfflineDriver(name string, config offlineDriverConfig) {
	offlineDriverConfigs[strings.ToLower(name)] = config
}

func offlineCommandContrast(command byte, args func(uint8) []byte) offlineContrastConfig {
	return offlineContrastConfig{kind: "command", data: func(level uint8) []byte {
		return append([]byte{command}, args(level)...)
	}}
}

func offlineDataContrast(data func(uint8) []byte) offlineContrastConfig {
	return offlineContrastConfig{kind: "data", data: data}
}

func reverseContrast(command byte, args func(uint8) []byte) offlineContrastConfig {
	return offlineCommandContrast(reverseBits(command), func(level uint8) []byte {
		out := args(level)
		for i := range out {
			out[i] = reverseBits(out[i])
		}
		return out
	})
}

func singleByte(level uint8) []byte { return []byte{level} }
