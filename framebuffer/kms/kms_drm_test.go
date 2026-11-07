//go:build linux || freebsd

package kms

import (
	"errors"
	"strings"
	"testing"
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
