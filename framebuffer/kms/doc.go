/*
Package kms opens DRM/KMS displays with dumb scanout buffers on Linux and
FreeBSD amd64, selecting a connected output and display mode.
*/
package kms

import "errors"

var (
	// ErrNoConnector indicates that no connected output matches the configuration.
	ErrNoConnector = errors.New("kms: no suitable connected connector")
	// ErrNotSupported indicates that an operation or platform is unsupported.
	ErrNotSupported = errors.New("kms: operation not supported")
)

// Config selects a DRM card, connected connector, and display mode. Device
// defaults to /dev/dri/card0. Connector accepts names such as HDMI-A-1. When
// Width and Height are nonzero, the matching mode is selected exactly; when
// both are zero, the connector's preferred mode is used.
type Config struct {
	// Device is the DRM card path; it defaults to /dev/dri/card0.
	Device string
	// Connector selects an output by name, such as HDMI-A-1.
	Connector string
	// Width and Height select an exact display mode when both are positive.
	Width, Height int
}
