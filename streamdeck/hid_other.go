//go:build !linux && (!darwin || ios)

package streamdeck

func enumerateHID() ([]hidIdentity, error) {
	return nil, ErrUnsupported
}

func resolvePath(path string) (string, error) {
	return path, nil
}

func openHID(string, uint16) (transport, error) {
	return nil, ErrUnsupported
}
