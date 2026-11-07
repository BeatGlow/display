//go:build linux

package fbdev

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"testing"

	"github.com/BeatGlow/display/v2/pixel"
)

func TestLinuxParseColorModel(t *testing.T) {
	tests := []struct {
		name  string
		info  linuxVarScreenInfo
		model color.Model
		image string
	}{
		{
			name: "RGB555",
			info: linuxVarScreenInfo{
				BitsPerPixel: 15,
				Red:          linuxBitField{Offset: 10, Length: 5},
				Green:        linuxBitField{Offset: 5, Length: 5},
				Blue:         linuxBitField{Length: 5},
			},
			model: pixel.CRGB15Model,
			image: "*pixel.CRGB15Image",
		},
		{
			name: "BGR555",
			info: linuxVarScreenInfo{
				BitsPerPixel: 15,
				Red:          linuxBitField{Length: 5},
				Green:        linuxBitField{Offset: 5, Length: 5},
				Blue:         linuxBitField{Offset: 10, Length: 5},
			},
			model: pixel.CBGR15Model,
			image: "*pixel.CBGR15Image",
		},
		{
			name: "RGB565",
			info: linuxVarScreenInfo{
				BitsPerPixel: 16,
				Red:          linuxBitField{Offset: 11, Length: 5},
				Green:        linuxBitField{Offset: 5, Length: 6},
				Blue:         linuxBitField{Length: 5},
			},
			model: pixel.CRGB16Model,
			image: "*pixel.CRGB16Image",
		},
		{
			name: "BGR565",
			info: linuxVarScreenInfo{
				BitsPerPixel: 16,
				Red:          linuxBitField{Length: 5},
				Green:        linuxBitField{Offset: 5, Length: 6},
				Blue:         linuxBitField{Offset: 11, Length: 5},
			},
			model: pixel.CBGR16Model,
			image: "*pixel.CBGR16Image",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, order, err := linuxParseColorModel(&tt.info)
			if err != nil {
				t.Fatal(err)
			}
			if model != tt.model || order != binary.NativeEndian {
				t.Fatalf("got model %v and order %v; want %v and native order", model, order, tt.model)
			}
			fb := &linuxFrameBuffer{model: model, order: order}
			fb.setImage()
			if got := fmt.Sprintf("%T", fb.Image); got != tt.image {
				t.Fatalf("image type %s; want %s", got, tt.image)
			}
		})
	}
}

func TestLinuxParseColorModelRejectsUnsupportedLayouts(t *testing.T) {
	for _, info := range []linuxVarScreenInfo{
		{BitsPerPixel: 24},
		{
			BitsPerPixel: 16,
			Red:          linuxBitField{Offset: 11, Length: 5, MsbRight: 1},
			Green:        linuxBitField{Offset: 5, Length: 6},
			Blue:         linuxBitField{Length: 5},
		},
	} {
		if _, _, err := linuxParseColorModel(&info); err == nil {
			t.Fatalf("accepted unsupported screen info: %+v", info)
		}
	}
}

func TestSetMappingUsesLineLengthAndOffsets(t *testing.T) {
	info := linuxVarScreenInfo{
		Xres: 2, Yres: 2, XresVirtual: 4, YresVirtual: 3,
		Xoffset: 1, Yoffset: 1, BitsPerPixel: 16,
	}
	fb := &linuxFrameBuffer{
		info:       linuxFrameBufferInfo{SmemLen: 36, LineLength: 12},
		screenInfo: info,
		model:      pixel.CRGB16Model,
		order:      binary.NativeEndian,
	}
	mapped := make([]byte, 36)
	if err := fb.configureImage(); err != nil {
		t.Fatal(err)
	}
	if err := fb.setMapping(mapped); err != nil {
		t.Fatal(err)
	}
	if got, want := fb.Bounds().Dx(), 2; got != want {
		t.Fatalf("width %d; want %d", got, want)
	}
	if got, want := fb.Bounds().Dy(), 2; got != want {
		t.Fatalf("height %d; want %d", got, want)
	}
	if fb.Stride != 12 {
		t.Fatalf("stride %d; want framebuffer line length 12", fb.Stride)
	}
	if _, ok := fb.Image.(*pixel.CRGB16Image); !ok {
		t.Fatalf("image type %T; want *pixel.CRGB16Image", fb.Image)
	}
	fb.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	if got, want := binary.NativeEndian.Uint16(mapped[14:16]), uint16(0xf800); got != want {
		t.Fatalf("mapped first pixel %#04x; want %#04x", got, want)
	}
}

func TestConfigureImageRejectsInvalidMetadata(t *testing.T) {
	fb := &linuxFrameBuffer{
		info: linuxFrameBufferInfo{SmemLen: 8, LineLength: 4},
		screenInfo: linuxVarScreenInfo{
			Xres: 3, Yres: 2, XresVirtual: 3, YresVirtual: 2, BitsPerPixel: 16,
		},
	}
	if err := fb.configureImage(); err == nil {
		t.Fatal("accepted a line shorter than the active row")
	}
}
