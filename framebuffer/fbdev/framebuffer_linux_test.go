//go:build linux

package fbdev

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/pixel"
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

func TestFramebufferRotationCoordinates(t *testing.T) {
	fb := &linuxFrameBuffer{info: linuxFrameBufferInfo{SmemLen: 24, LineLength: 8},
		screenInfo: linuxVarScreenInfo{Xres: 4, Yres: 3, XresVirtual: 4, YresVirtual: 3, BitsPerPixel: 16},
		model:      pixel.CRGB16Model, order: binary.LittleEndian}
	if err := fb.configureImage(); err != nil {
		t.Fatal(err)
	}
	if err := fb.setMapping(make([]byte, 24)); err != nil {
		t.Fatal(err)
	}
	for _, rotation := range []display.Rotation{display.NoRotation, display.Rotate90, display.Rotate180, display.Rotate270, display.Rotation(5)} {
		if err := fb.SetRotation(rotation); err != nil {
			t.Fatal(err)
		}
		want := image.Pt(4, 3)
		if rotation%4 == display.Rotate90 || rotation%4 == display.Rotate270 {
			want = image.Pt(3, 4)
		}
		if got := fb.Bounds().Size(); got != want {
			t.Fatalf("rotation %v bounds %v; want %v", rotation, got, want)
		}
		for y := 0; y < fb.Bounds().Dy(); y++ {
			for x := 0; x < fb.Bounds().Dx(); x++ {
				v := color.RGBA{R: uint8(1 + x + 16*y), G: 0, B: 0, A: 0xff}
				fb.Set(x, y, v)
				if got := fb.At(x, y); got != pixel.CRGB16Model.Convert(v) {
					t.Fatalf("rotation %v pixel (%d,%d) = %v, want %v", rotation, x, y, got, pixel.CRGB16Model.Convert(v))
				}
			}
		}
	}
}

func TestFramebufferCopyFromRotated(t *testing.T) {
	fb := &linuxFrameBuffer{info: linuxFrameBufferInfo{SmemLen: 24, LineLength: 8},
		screenInfo: linuxVarScreenInfo{Xres: 4, Yres: 3, XresVirtual: 4, YresVirtual: 3, BitsPerPixel: 16},
		model:      pixel.CRGB16Model, order: binary.LittleEndian}
	if err := fb.configureImage(); err != nil {
		t.Fatal(err)
	}
	if err := fb.setMapping(make([]byte, 24)); err != nil {
		t.Fatal(err)
	}
	for _, rotation := range []display.Rotation{display.Rotate90, display.Rotate180, display.Rotate270} {
		if err := fb.SetRotation(rotation); err != nil {
			t.Fatal(err)
		}
		bounds := fb.Bounds()
		src := image.NewRGBA(image.Rect(7, 9, 7+bounds.Dx(), 9+bounds.Dy()))
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				src.SetRGBA(7+x, 9+y, color.RGBA{R: uint8(40 + x*7), G: uint8(20 + y*11), B: 123, A: 255})
			}
		}
		if err := fb.CopyFrom(src); err != nil {
			t.Fatal(err)
		}
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				if got, want := fb.At(x, y), pixel.CRGB16Model.Convert(src.At(7+x, 9+y)); got != want {
					t.Fatalf("rotation %v CopyFrom pixel (%d,%d) = %v; want %v", rotation, x, y, got, want)
				}
			}
		}
	}
	if err := fb.CopyFrom(nil); err == nil {
		t.Fatal("CopyFrom(nil) succeeded")
	}
	if err := fb.CopyFrom(image.NewRGBA(image.Rect(0, 0, 1, 1))); err == nil {
		t.Fatal("CopyFrom accepted a mismatched image size")
	}
}

func TestPackRGBA16UsesByteOrder(t *testing.T) {
	src := image.NewRGBA(image.Rect(5, 7, 6, 8))
	src.SetRGBA(5, 7, color.RGBA{R: 83, G: 151, B: 29, A: 255})
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian, binary.NativeEndian} {
		t.Run(fmt.Sprintf("%T", order), func(t *testing.T) {
			packed := make([]byte, 2)
			packRGBA16(packed, src, src.Bounds(), pixel.FormatCRGB16, isLittleEndian(order))
			var want [2]byte
			order.PutUint16(want[:], 0x54a3)
			if string(packed) != string(want[:]) {
				t.Fatalf("packed % x; want % x", packed, want)
			}
		})
	}
}

func BenchmarkFramebufferSetRotation(b *testing.B) {
	fb := &linuxFrameBuffer{info: linuxFrameBufferInfo{SmemLen: 800 * 480 * 2, LineLength: 800 * 2},
		screenInfo: linuxVarScreenInfo{Xres: 800, Yres: 480, XresVirtual: 800, YresVirtual: 480, BitsPerPixel: 16},
		model:      pixel.CRGB16Model, order: binary.LittleEndian}
	if err := fb.configureImage(); err != nil {
		b.Fatal(err)
	}
	if err := fb.setMapping(make([]byte, 800*480*2)); err != nil {
		b.Fatal(err)
	}
	c := color.RGBA{R: 0x87, G: 0x43, B: 0x21, A: 0xff}
	for _, rotation := range []display.Rotation{display.NoRotation, display.Rotate90, display.Rotate180, display.Rotate270} {
		b.Run(rotation.String(), func(b *testing.B) {
			b.ReportAllocs()
			bounds := fb.Bounds()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				fb.Set(n%bounds.Dx(), (n/bounds.Dx())%bounds.Dy(), c)
			}
		})
		if err := fb.SetRotation(rotation); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFramebufferCopyFrom(b *testing.B) {
	const width, height = 800, 480
	fb := &linuxFrameBuffer{info: linuxFrameBufferInfo{SmemLen: width * height * 2, LineLength: width * 2},
		screenInfo: linuxVarScreenInfo{Xres: width, Yres: height, XresVirtual: width, YresVirtual: height, BitsPerPixel: 16},
		model:      pixel.CRGB16Model, order: binary.LittleEndian}
	if err := fb.configureImage(); err != nil {
		b.Fatal(err)
	}
	if err := fb.setMapping(make([]byte, width*height*2)); err != nil {
		b.Fatal(err)
	}
	for _, rotation := range []display.Rotation{display.NoRotation, display.Rotate90, display.Rotate180, display.Rotate270} {
		if err := fb.SetRotation(rotation); err != nil {
			b.Fatal(err)
		}
		src := image.NewRGBA(image.Rect(0, 0, fb.Bounds().Dx(), fb.Bounds().Dy()))
		b.Run(rotation.String(), func(b *testing.B) {
			b.SetBytes(width * height * 4)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := fb.CopyFrom(src); err != nil {
					b.Fatal(err)
				}
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
