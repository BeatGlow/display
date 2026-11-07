package ioctl

import "testing"

func TestFreeBSDRequestEncoding(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode Mode
		size uint16
		cmd  uintptr
		want Command
	}{
		{name: "DRM IOWR", mode: Read | Write, size: 64, cmd: 0x64a0, want: 0xc04064a0},
		{name: "DRM IOW", mode: Write, size: 4, cmd: 0x64af, want: 0x800464af},
		{name: "DRM IO", mode: None, cmd: 0x641e, want: 0x2000641e},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeFreeBSD(tt.mode, tt.size, tt.cmd); got != tt.want {
				t.Fatalf("request %#x; want %#x", got, tt.want)
			}
		})
	}
}
