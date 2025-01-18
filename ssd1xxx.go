package display

const (
	setLowColumn          = 0x00
	setHighColumn         = 0x10
	setMemoryMode         = 0x20
	setColumnAddr         = 0x21
	setPageAddr           = 0x22
	setStartLine          = 0x40
	setContrast           = 0x81
	setChargePump         = 0x8D
	setRemap              = 0xA0
	setSegmentRemap       = 0xA1
	setDisplayAllOnResume = 0xA4
	setDisplayAllOn       = 0xA5
	setNormalDisplay      = 0xA6
	setInvertDisplay      = 0xA7
	setMultiplexRatio     = 0xA8
	setDisplayOff         = 0xAE
	setDisplayOn          = 0xAF
	setComScanInc         = 0xC0
	setComScanDec         = 0xC8
	setDisplayOffset      = 0xD3
	setDisplayClockDiv    = 0xD5
	setPrecharge          = 0xD9
	setComPins            = 0xDA
	setVComDetect         = 0xDB
	externalVCC           = 0x1
	switchCapVCC          = 0x2
)
