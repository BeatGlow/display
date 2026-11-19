// Package scope renders animated SVG outlines with a simulated X-Y CRT beam.
package scope

import (
	"errors"
	"image"
	"image/color"
	"io"
	"math"
	"slices"
	"time"
)

// Config controls signal generation and the phosphor response. Use DefaultConfig.
type Config struct {
	SampleRate                      int
	DrawSpeed                       float64
	LowpassHz, HighpassHz           float64
	MaxDVdt                         float64
	NoiseSigma, HumAmplitude, HumHz float64
	BeamRadius                      float64
	Persistence                     time.Duration
	Exposure, CRTBulge              float64
	Graticule                       bool
}

// DefaultConfig returns settings for a small embedded display. A zero beam
// radius is resolved from display height and capped at two pixels. Explicit
// BeamRadius values must be between 0.1 and two pixels.
func DefaultConfig() Config {
	return Config{SampleRate: 192000, DrawSpeed: 15, LowpassHz: 22000, MaxDVdt: 6000,
		NoiseSigma: 0.0002, HumHz: 50, Persistence: 40 * time.Millisecond, Exposure: 1, CRTBulge: 0.25, Graticule: true}
}

type remapPixel struct {
	index  [4]int32
	weight [4]float32
}
type point struct{ x, y float64 }

func mix(a, b point, t float64) point { return point{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t} }

// Renderer owns all buffers. Its methods must be called serially. Recreate it
// when display bounds or static configuration change.
type Renderer struct {
	bounds                             image.Rectangle
	cfg                                Config
	width, height                      int
	charge, scratch, bloom, work, base []float32
	remap                              []remapPixel
	lut, grayLUT                       [256]color.Color
	colors                             [256]color.RGBA
	grayValues                         [256]uint8
	monoLUT                            [2]color.Color
	x, y, frameX, frameY               []float32
	cursor, frameCount                 int
	pending                            bool
	frameDuration                      time.Duration
	fractional                         float64
	amp                                amplifier
	last                               point
	clock                              float64
	rng                                uint64
	kernel                             [49]float32
}

// NewRenderer allocates buffers and precomputes optics and color tables.
func NewRenderer(bounds image.Rectangle, cfg Config) (*Renderer, error) {
	w, h := bounds.Dx(), bounds.Dy()
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if w < 2 || h < 2 || w > 4_194_304/h {
		return nil, errors.New("scope: bounds must contain 2 to 4194304 pixels with dimensions at least two")
	}
	values := []float64{cfg.DrawSpeed, cfg.LowpassHz, cfg.HighpassHz, cfg.MaxDVdt, cfg.NoiseSigma, cfg.HumAmplitude, cfg.HumHz, cfg.BeamRadius, cfg.Exposure, cfg.CRTBulge}
	for _, v := range values {
		if !finite(v) {
			return nil, errors.New("scope: configuration values must be finite")
		}
	}
	if cfg.SampleRate < 1000 || cfg.SampleRate > 384000 || cfg.DrawSpeed <= 0 || cfg.LowpassHz <= 0 || cfg.LowpassHz >= float64(cfg.SampleRate)/2 || cfg.HighpassHz < 0 || cfg.HighpassHz >= float64(cfg.SampleRate)/2 || cfg.MaxDVdt <= 0 || cfg.NoiseSigma < 0 || cfg.NoiseSigma > 0.01 || cfg.HumAmplitude < 0 || cfg.HumAmplitude > 0.1 || cfg.HumHz < 0 || cfg.HumHz >= float64(cfg.SampleRate)/2 || cfg.BeamRadius < 0 || (cfg.BeamRadius > 0 && cfg.BeamRadius < 0.1) || cfg.BeamRadius > 2 || cfg.Exposure < 0 || cfg.Exposure > 100 || cfg.CRTBulge < 0 || cfg.CRTBulge > 1 || cfg.Persistence <= 0 {
		return nil, errors.New("scope: invalid configuration")
	}
	if cfg.BeamRadius == 0 {
		cfg.BeamRadius = math.Min(2, math.Max(0.45, float64(h)*0.012))
	}
	n := w * h
	r := &Renderer{bounds: bounds, cfg: cfg, width: w, height: h, charge: make([]float32, n), scratch: make([]float32, n), bloom: make([]float32, n), work: make([]float32, n), remap: make([]remapPixel, n), frameX: make([]float32, cfg.SampleRate), frameY: make([]float32, cfg.SampleRate), rng: 1}
	for i := range r.remap {
		x, y := float64(i%w), float64(i/w)
		u, v := 2*x/float64(w-1)-1, 2*y/float64(h-1)-1
		a := float64(w-1) / float64(h-1)
		rr := (u*a)*(u*a) + v*v
		s := 1 + cfg.CRTBulge*(0.18*rr+0.04*rr*rr)
		sx, sy := (u*s+1)*float64(w-1)/2, (v*s+1)*float64(h-1)/2
		if sx < 0 || sy < 0 || sx > float64(w-1) || sy > float64(h-1) {
			r.remap[i].index = [4]int32{-1, -1, -1, -1}
			continue
		}
		ix, iy := int(sx), int(sy)
		fx, fy := float32(sx-float64(ix)), float32(sy-float64(iy))
		r.remap[i] = remapPixel{[4]int32{int32(iy*w + ix), int32(iy*w + min(w-1, ix+1)), int32(min(h-1, iy+1)*w + ix), int32(min(h-1, iy+1)*w + min(w-1, ix+1))}, [4]float32{(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy}}
	}
	var sum float64
	for y := -3; y <= 3; y++ {
		for x := -3; x <= 3; x++ {
			v := math.Exp(-float64(x*x+y*y) / (2 * cfg.BeamRadius * cfg.BeamRadius))
			r.kernel[(y+3)*7+x+3] = float32(v)
			sum += v
		}
	}
	for i := range r.kernel {
		r.kernel[i] /= float32(sum)
	}
	for i := range r.colors {
		t := float64(i) / 255
		e := t / (1.001 - t)
		white := math.Min(1, math.Max(0, (e-1)/1.5))
		mid := math.Min(1, e)
		rgb := [3]float64{0.05 * mid, 1, 0.25 * mid}
		var out color.RGBA
		out.A = 255
		channels := [3]*uint8{&out.R, &out.G, &out.B}
		for c := range rgb {
			v := rgb[c] + white*(1-rgb[c])
			v = e * v / (1 + e*v)
			*channels[c] = uint8(math.Round(255 * math.Pow(v, 1/2.2)))
		}
		r.colors[i] = out
		r.lut[i] = out
		lum := uint8(math.Round(255 * math.Pow(math.Min(1, t/0.75), 1/1.8)))
		r.grayValues[i] = lum
		r.grayLUT[i] = color.Gray{Y: lum}
	}
	r.monoLUT = [2]color.Color{color.Black, color.White}
	r.initScreen()
	return r, nil
}

// CompileSVG replaces the repeating signal. Compilation may allocate; frame
// methods reuse buffers. An error preserves the previously compiled drawing.
func (r *Renderer) CompileSVG(input io.Reader) error {
	if input == nil {
		return errors.New("scope: nil SVG input")
	}
	paths, err := compilePaths(input, r.width, r.height)
	if err != nil {
		return err
	}
	aspect := float64(r.height-1) / float64(r.width-1)
	distance := func(a, b point) float64 { return math.Hypot(a.x-b.x, (a.y-b.y)*aspect) / 2 }
	for i := 1; i < len(paths); i++ {
		from := paths[i-1][len(paths[i-1])-1]
		best, d, rev := i, math.Inf(1), false
		for j := i; j < len(paths); j++ {
			p := paths[j]
			if v := distance(from, p[0]); v < d {
				best, d, rev = j, v, false
			}
			if v := distance(from, p[len(p)-1]); v < d {
				best, d, rev = j, v, true
			}
		}
		paths[i], paths[best] = paths[best], paths[i]
		if rev {
			slices.Reverse(paths[i])
		}
	}
	x, y := make([]float32, 0, 4096), make([]float32, 0, 4096)
	appendPoint := func(p point) { x = append(x, float32(p.x)); y = append(y, float32(p.y)) }
	step := r.cfg.DrawSpeed / float64(r.cfg.SampleRate)
	for i, p := range paths {
		appendPoint(p[0])
		next := step
		for j := 1; j < len(p); j++ {
			a, b := p[j-1], p[j]
			length := distance(a, b)
			if length == 0 {
				continue
			}
			if length/step > float64(2_000_000-len(x)) {
				return errors.New("scope: signal exceeds two million samples")
			}
			for next < length {
				appendPoint(mix(a, b, next/length))
				next += step
			}
			next -= length
			stop := j == len(p)-1
			if !stop {
				d := p[j+1]
				ax, ay := b.x-a.x, (b.y-a.y)*aspect
				bx, by := d.x-b.x, (d.y-b.y)*aspect
				stop = ax*bx+ay*by < math.Sqrt(0.5)*math.Hypot(ax, ay)*math.Hypot(bx, by)
			}
			if stop {
				appendPoint(b)
				appendPoint(b)
				next = step
			}
		}
		appendPoint(paths[(i+1)%len(paths)][0])
	}
	if len(x) > 2_000_000 {
		return errors.New("scope: signal exceeds two million samples")
	}
	if len(x) == 0 {
		return errors.New("scope: SVG contains no drawable paths")
	}
	r.x, r.y = x, y
	r.cursor = 0
	r.pending = false
	r.frameCount = 0
	r.fractional = 0
	r.clock = 0
	r.rng = 1
	clear(r.charge)
	r.last = point{float64(x[0]), float64(y[0])}
	r.amp = newAmplifier(r.cfg, r.last)
	return nil
}
