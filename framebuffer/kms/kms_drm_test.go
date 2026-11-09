//go:build linux || freebsd

package kms

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/internal/rotation"
)

func TestChooseConnector(t *testing.T) {
	preferred := modeInfo{hDisplay: 1920, vDisplay: 1080, typ: drmModeTypePreferred}
	other := modeInfo{hDisplay: 1280, vDisplay: 720}
	connected := connectorInfo{
		kernel: modeConnector{connection: drmModeConnected},
		modes:  []modeInfo{other, preferred},
		name:   "HDMI-A-1",
	}
	disconnected := connectorInfo{
		kernel: modeConnector{connection: 0},
		modes:  []modeInfo{preferred},
		name:   "DP-1",
	}

	tests := []struct {
		name        string
		connectors  []connectorInfo
		config      *Config
		wantName    string
		wantMode    modeInfo
		wantErr     error
		wantErrText string
	}{
		{
			name:       "preferred mode",
			connectors: []connectorInfo{disconnected, connected},
			config:     &Config{},
			wantName:   "HDMI-A-1",
			wantMode:   preferred,
		},
		{
			name:       "named connector and exact mode",
			connectors: []connectorInfo{connected},
			config:     &Config{Connector: "hdmi-a-1", Width: 1280, Height: 720},
			wantName:   "HDMI-A-1",
			wantMode:   other,
		},
		{
			name:       "no connected connector",
			connectors: []connectorInfo{disconnected},
			config:     &Config{},
			wantErr:    ErrNoConnector,
		},
		{
			name: "ambiguous connected connectors",
			connectors: []connectorInfo{
				connected,
				{kernel: modeConnector{connection: drmModeConnected}, modes: []modeInfo{preferred}, name: "DP-1"},
			},
			config:      &Config{},
			wantErrText: "select one with Config.Connector",
		},
		{
			name:        "requested mode unavailable",
			connectors:  []connectorInfo{connected},
			config:      &Config{Connector: "HDMI-A-1", Width: 800, Height: 600},
			wantErrText: "has no 800x600 mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector, mode, err := chooseConnector(tt.connectors, tt.config)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("chooseConnector() error = %v; want error wrapping %v", err, tt.wantErr)
			}
			if tt.wantErrText != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrText)) {
				t.Fatalf("chooseConnector() error = %v; want it to contain %q", err, tt.wantErrText)
			}
			if tt.wantErr != nil || tt.wantErrText != "" {
				if err == nil {
					t.Fatal("chooseConnector() error = nil; want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("chooseConnector() error = %v", err)
			}
			if connector.name != tt.wantName || mode != tt.wantMode {
				t.Fatalf("chooseConnector() = (%q, %dx%d); want (%q, %dx%d)", connector.name, mode.hDisplay, mode.vDisplay, tt.wantName, tt.wantMode.hDisplay, tt.wantMode.vDisplay)
			}
		})
	}
}

func TestSetRotation(t *testing.T) {
	native := image.Rect(0, 0, 4, 3)
	d := &displayDevice{mode: modeInfo{hDisplay: 4, vDisplay: 3}, shadow: image.NewRGBA(native)}
	for y := 0; y < native.Dy(); y++ {
		for x := 0; x < native.Dx(); x++ {
			d.shadow.SetRGBA(x, y, color.RGBA{R: uint8(x + 1), G: uint8(y + 1), A: 0xff})
		}
	}
	physical := image.NewRGBA(native)
	copy(physical.Pix, d.shadow.Pix)
	for _, r := range []display.Rotation{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotation(4)} {
		if err := d.SetRotation(r); err != nil {
			t.Fatal(err)
		}
		bounds := rotation.Bounds(native, r)
		if got := d.Bounds(); got != bounds {
			t.Fatalf("rotation %v bounds = %v; want %v", r, got, bounds)
		}
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				px, py := rotation.Point(native, r, x, y)
				if got, want := d.At(x, y), physical.At(px, py); got != want {
					t.Fatalf("rotation %v pixel (%d,%d) = %v; want native (%d,%d) %v", r, x, y, got, px, py, want)
				}
			}
		}
	}
	if err := d.SetRotation(display.Rotate90); err != nil {
		t.Fatal(err)
	}
	d.Set(0, 0, color.RGBA{B: 0xff, A: 0xff})
	if got, want := d.At(0, 0), (color.RGBA{B: 0xff, A: 0xff}); got != want {
		t.Fatalf("rotated Set/At = %v; want %v", got, want)
	}
}

func TestFreeBufferIndex(t *testing.T) {
	tests := []struct {
		name         string
		front        int
		pending      bool
		pendingIndex int
		want         int
	}{
		{name: "initial", front: 0, want: 1},
		{name: "flip pending", front: 0, pending: true, pendingIndex: 1, want: 2},
		{name: "previous flip complete", front: 1, want: 0},
		{name: "pending third buffer", front: 1, pending: true, pendingIndex: 2, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &displayDevice{front: tt.front, pendingFlip: tt.pending, pendingIndex: tt.pendingIndex}
			if got := d.freeBufferIndex(); got != tt.want {
				t.Fatalf("freeBufferIndex() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestConnectorName(t *testing.T) {
	tests := []struct {
		typ, id uint32
		want    string
	}{
		{typ: drmModeConnectorHDMIA, id: 2, want: "HDMI-A-2"},
		{typ: drmModeConnectorDisplayPort, id: 1, want: "DP-1"},
		{typ: 999, id: 3, want: "Connector-999-3"},
	}
	for _, tt := range tests {
		if got := connectorName(tt.typ, tt.id); got != tt.want {
			t.Errorf("connectorName(%d, %d) = %q; want %q", tt.typ, tt.id, got, tt.want)
		}
	}
}
