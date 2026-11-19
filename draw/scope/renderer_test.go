package scope_test

import (
	"bytes"
	"image"
	"image/draw"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/BeatGlow/display/draw/scope"
	"github.com/BeatGlow/display/pixel"
)

const splashSVG = `<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 40L20 0L40 40"/><rect x="60" y="5" width="40" height="30"/><circle cx="130" cy="20" r="15"/></svg>`

const frameTime = time.Second / 60

func compiledRenderer(tb testing.TB, bounds image.Rectangle) *scope.Renderer {
	tb.Helper()
	r, err := scope.NewRenderer(bounds, scope.DefaultConfig())
	if err != nil {
		tb.Fatal(err)
	}
	if err := r.CompileSVG(strings.NewReader(splashSVG)); err != nil {
		tb.Fatal(err)
	}
	return r
}

func frameTargets() []struct {
	name  string
	image draw.Image
} {
	return []struct {
		name  string
		image draw.Image
	}{
		{"RGBA", image.NewRGBA(image.Rect(0, 0, 400, 128))},
		{"CRGB16", pixel.NewCRGB16Image(400, 128)},
		{"Gray4", pixel.NewGray4Image(400, 128)},
		{"Mono", pixel.NewMonoImage(400, 128)},
		{"RGBA-offset", image.NewRGBA(image.Rect(7, 11, 407, 139))},
	}
}

func TestSplashFrameNeedsNoHeapAllocationAndReachesEveryColorMode(t *testing.T) {
	for _, target := range frameTargets() {
		t.Run(target.name, func(t *testing.T) {
			r := compiledRenderer(t, target.image.Bounds())
			var renderErr error
			allocations := testing.AllocsPerRun(20, func() {
				r.UpdatePhysics(frameTime)
				renderErr = r.RenderTo(target.image)
			})
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			if allocations != 0 {
				t.Fatalf("frame allocated %g heap objects", allocations)
			}
			var lit, dark, colored int
			bounds := target.image.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					r, g, b, _ := target.image.At(x, y).RGBA()
					if max(r, g, b) > 0x4000 {
						lit++
					} else {
						dark++
					}
					if g > r+0x1000 && g > b+0x1000 {
						colored++
					}
				}
			}
			if lit < 30 || dark < bounds.Dx()*bounds.Dy()/2 {
				t.Fatalf("trace disappeared or flooded the faceplate: lit=%d dark=%d", lit, dark)
			}
			if strings.HasPrefix(target.name, "RGBA") || target.name == "CRGB16" {
				if colored == 0 {
					t.Fatal("color display lost its green phosphor")
				}
			} else if colored != 0 {
				t.Fatal("luminance display received colored pixels")
			}
		})
	}
}

func TestSplashRejectsMalformedDrawingAndMismatchedDisplayBounds(t *testing.T) {
	r := compiledRenderer(t, image.Rect(0, 0, 128, 64))
	r.UpdatePhysics(frameTime)
	before := image.NewRGBA(image.Rect(0, 0, 128, 64))
	if err := r.RenderTo(before); err != nil {
		t.Fatal(err)
	}
	for _, svg := range []string{"<svg>", `<svg><path d="M0 0 C"/></svg>`, "<svg/>"} {
		t.Run(svg, func(t *testing.T) {
			if err := r.CompileSVG(strings.NewReader(svg)); err == nil {
				t.Fatal("malformed or empty SVG was accepted")
			}
		})
	}
	after := image.NewRGBA(before.Bounds())
	if err := r.RenderTo(after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Pix, after.Pix) {
		t.Fatal("failed compilation changed the active splash")
	}
	if err := r.RenderTo(image.NewRGBA(image.Rect(0, 0, 64, 32))); err == nil {
		t.Fatal("renderer accepted a destination with different bounds")
	}
}

func TestRendererRejectsInvalidOpticsAndSignalConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*scope.Config)
	}{
		{"sample-rate", func(c *scope.Config) { c.SampleRate = 0 }},
		{"bandwidth", func(c *scope.Config) { c.LowpassHz = float64(c.SampleRate) }},
		{"bulge", func(c *scope.Config) { c.CRTBulge = 1.1 }},
		{"non-finite-exposure", func(c *scope.Config) { c.Exposure = math.NaN() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := scope.DefaultConfig()
			tc.change(&cfg)
			if _, err := scope.NewRenderer(image.Rect(0, 0, 128, 64), cfg); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
	if _, err := scope.NewRenderer(image.Rectangle{}, scope.DefaultConfig()); err == nil {
		t.Fatal("empty faceplate was accepted")
	}
}

func BenchmarkRenderFrame(b *testing.B) {
	for _, target := range frameTargets() {
		b.Run(target.name, func(b *testing.B) {
			r := compiledRenderer(b, target.image.Bounds())
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				r.UpdatePhysics(frameTime)
				if err := r.RenderTo(target.image); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestSeparateSignalStepMatchesCombinedPhysics(t *testing.T) {
	bounds := image.Rect(0, 0, 128, 64)
	combined, separate := compiledRenderer(t, bounds), compiledRenderer(t, bounds)
	a, b := image.NewRGBA(bounds), image.NewRGBA(bounds)
	for range 3 {
		combined.UpdatePhysics(frameTime)
		separate.StepSignal(frameTime)
		separate.StepSignal(0)
		separate.StepSignal(-time.Second)
		separate.UpdatePhysics(frameTime)
	}
	if err := combined.RenderTo(a); err != nil {
		t.Fatal(err)
	}
	if err := separate.RenderTo(b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("preparing the signal twice advanced the drawing")
	}
}

func TestIdleGraticuleRemainsVisibleOnSmallDisplay(t *testing.T) {
	bounds := image.Rect(0, 0, 128, 64)
	cfg := scope.DefaultConfig()
	cfg.CRTBulge = 0
	grid, err := scope.NewRenderer(bounds, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Graticule = false
	blank, err := scope.NewRenderer(bounds, cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, b := image.NewRGBA(bounds), image.NewRGBA(bounds)
	if err := grid.RenderTo(a); err != nil {
		t.Fatal(err)
	}
	if err := blank.RenderTo(b); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("graticule disappeared in LUT quantization")
	}
}
