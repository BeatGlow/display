package streamdeck

import "image"

type modelSpec struct {
	model        Model
	newProtocol  func() protocol
	serialReport serialReport
}

// Product IDs are the models registered by Benjamin's Stream Deck driver.
var models = map[uint16]modelSpec{
	0x0060:        keyModel("Stream Deck Original", 5, 3, 72, newOriginalProtocol, shortLegacySerial),
	0x0063:        keyModel("Stream Deck Mini", 3, 2, 80, newMiniProtocol, shortLegacySerial),
	0x006c:        keyModel("Stream Deck XL", 8, 4, 96, newModernKeyProtocol, modernSerial),
	0x006d:        keyModel("Stream Deck 2019", 5, 3, 72, newModernKeyProtocol, modernSerial),
	0x0080:        keyModel("Stream Deck MK.2", 5, 3, 72, newModernKeyProtocol, modernSerial),
	plusProductID: {model: plusModel, newProtocol: func() protocol { return &plusProtocol{} }, serialReport: modernSerial},
	0x0090:        keyModel("Stream Deck Mini 2022", 3, 2, 80, newMiniProtocol, longLegacySerial),
}

func lookupModel(productID uint16) (modelSpec, bool) {
	spec, ok := models[productID]
	return spec, ok
}

func keyModel(name string, columns, rows, keySize int, newProtocol func(Model) protocol, serial serialReport) modelSpec {
	model := Model{
		Name:       name,
		KeyColumns: columns,
		KeyRows:    rows,
		KeySize:    image.Pt(keySize, keySize),
	}
	return modelSpec{
		model:        model,
		newProtocol:  func() protocol { return newProtocol(model) },
		serialReport: serial,
	}
}
