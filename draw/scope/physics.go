package scope

import (
	"math"
	"time"
)

type amplifier struct {
	gain, a1, a2, hp, maxStep              float64
	last, high, drive1, drive2, out1, out2 point
}

func newAmplifier(c Config, start point) amplifier {
	k := math.Tan(math.Pi * c.LowpassHz / float64(c.SampleRate))
	norm := 1 / (1 + math.Sqrt2*k + k*k)
	a := amplifier{gain: k * k * norm, a1: 2 * (k*k - 1) * norm, a2: (1 - math.Sqrt2*k + k*k) * norm, maxStep: math.Max(math.SmallestNonzeroFloat64, c.MaxDVdt/float64(c.SampleRate)), last: start, drive1: start, drive2: start, out1: start, out2: start}
	if c.HighpassHz > 0 {
		a.hp = math.Exp(-2 * math.Pi * c.HighpassHz / float64(c.SampleRate))
		a.high = start
	}
	return a
}
func (a *amplifier) apply(p point) point {
	if a.hp > 0 {
		a.high = point{a.hp * (a.high.x + p.x - a.last.x), a.hp * (a.high.y + p.y - a.last.y)}
		a.last, p = p, a.high
	}
	dx, dy := p.x-a.drive1.x, p.y-a.drive1.y
	d := math.Hypot(dx, dy)
	scale := 1.0
	// A bounded proportional vector step keeps both axes settling together.
	if d > a.maxStep {
		scale = a.maxStep / d
	}
	drive := point{a.drive1.x + scale*dx, a.drive1.y + scale*dy}
	out := point{a.gain*(drive.x+2*a.drive1.x+a.drive2.x) - a.a1*a.out1.x - a.a2*a.out2.x, a.gain*(drive.y+2*a.drive1.y+a.drive2.y) - a.a1*a.out1.y - a.a2*a.out2.y}
	a.drive2, a.drive1, a.out2, a.out1 = a.drive1, drive, a.out1, out
	return out
}
func (r *Renderer) random() float64 {
	// Two summed uniforms give a small, independent thermal-noise approximation.
	r.rng ^= r.rng << 13
	r.rng ^= r.rng >> 7
	r.rng ^= r.rng << 17
	a := float64(r.rng>>32) / 4294967296
	r.rng ^= r.rng << 13
	r.rng ^= r.rng >> 7
	r.rng ^= r.rng << 17
	return (a + float64(r.rng>>32)/4294967296 - 1) * 2.449489743
}

// StepSignal advances the repeating DAC and filters into preallocated buffers.
// A call processes at most one second, discarding longer elapsed pauses.
// UpdatePhysics consumes this prepared frame instead of advancing it again.
// Calling StepSignal repeatedly replaces the pending frame.
func (r *Renderer) StepSignal(dt time.Duration) {
	if dt <= 0 || len(r.x) == 0 {
		return
	}
	if r.pending && r.frameCount > 0 {
		r.last = point{float64(r.frameX[r.frameCount-1]), float64(r.frameY[r.frameCount-1])}
	}
	r.frameCount = 0
	r.pending = false
	dt = min(dt, time.Second)
	r.frameDuration = dt
	r.pending = true
	samples := dt.Seconds()*float64(r.cfg.SampleRate) + r.fractional
	n := min(len(r.frameX), int(samples))
	r.fractional = samples - float64(n)
	for i := 0; i < n; i++ {
		hum := r.cfg.HumAmplitude * math.Sin(2*math.Pi*r.cfg.HumHz*r.clock)
		p := point{float64(r.x[r.cursor]) + hum + r.cfg.NoiseSigma*r.random(), float64(r.y[r.cursor]) + hum + r.cfg.NoiseSigma*r.random()}
		q := r.amp.apply(p)
		r.frameX[i], r.frameY[i] = float32(q.x), float32(q.y)
		r.cursor++
		if r.cursor == len(r.x) {
			r.cursor = 0
		}
		r.clock += 1 / float64(r.cfg.SampleRate)
	}
	r.frameCount = n
}

// UpdatePhysics advances the DAC and deposits charge after exponential decay.
// It performs no heap allocation. Negative and zero elapsed time do nothing.
func (r *Renderer) UpdatePhysics(dt time.Duration) {
	if dt <= 0 {
		return
	}
	if !r.pending {
		r.StepSignal(dt)
	}
	elapsed := min(dt, time.Second)
	if r.pending {
		elapsed = r.frameDuration
	}
	decay := float32(math.Exp(-elapsed.Seconds() / r.cfg.Persistence.Seconds()))
	for i := range r.charge {
		r.charge[i] *= decay
	}
	for i := 0; i < r.frameCount; i++ {
		q := point{float64(r.frameX[i]), float64(r.frameY[i])}
		r.integrate(r.last, q)
		r.last = q
	}
	r.pending = false
	r.frameCount = 0
}
func (r *Renderer) integrate(a, b point) {
	hw, hh := float64(r.width-1)/2, float64(r.height-1)/2
	ax, ay := (a.x+1)*hw, (a.y+1)*hh
	bx, by := (b.x+1)*hw, (b.y+1)*hh
	distance := math.Hypot(bx-ax, by-ay)
	steps := max(1, int(math.Ceil(distance*2)))
	// Offscreen transients are clipped by each bounded deposition. The normalized
	// signal and stable filters keep segment subdivision within a small screen span.
	steps = min(steps, 4*(r.width+r.height))
	energy := float32(68 * float64(r.width) / (float64(r.cfg.SampleRate) * float64(steps)))
	for i := 0; i < steps; i++ {
		t := (float64(i) + 0.5) / float64(steps)
		x, y := ax+(bx-ax)*t, ay+(by-ay)*t
		r.deposit(int(math.Floor(x+0.5)), int(math.Floor(y+0.5)), energy)
	}
}
func (r *Renderer) deposit(x, y int, energy float32) {
	for dy := -3; dy <= 3; dy++ {
		yy := y + dy
		if yy < 0 || yy >= r.height {
			continue
		}
		for dx := -3; dx <= 3; dx++ {
			xx := x + dx
			if xx < 0 || xx >= r.width {
				continue
			}
			i := yy*r.width + xx
			r.charge[i] = min(32, r.charge[i]+energy*r.kernel[(dy+3)*7+dx+3])
		}
	}
}

// boxBlur uses running sums, so its work does not grow with the blur radius.
func (r *Renderer) boxBlur(src, dst []float32, radius int) {
	radius = max(1, radius)
	factor := float32(1) / float32(2*radius+1)
	for y := 0; y < r.height; y++ {
		row := y * r.width
		sum := float32(0)
		for x := 0; x <= min(radius, r.width-1); x++ {
			sum += src[row+x]
		}
		for x := 0; x < r.width; x++ {
			r.scratch[row+x] = sum * factor
			if add := x + radius + 1; add < r.width {
				sum += src[row+add]
			}
			if sub := x - radius; sub >= 0 {
				sum -= src[row+sub]
			}
		}
	}
	for x := 0; x < r.width; x++ {
		sum := float32(0)
		for y := 0; y <= min(radius, r.height-1); y++ {
			sum += r.scratch[y*r.width+x]
		}
		for y := 0; y < r.height; y++ {
			dst[y*r.width+x] = max(0, sum*factor)
			if add := y + radius + 1; add < r.height {
				sum += r.scratch[add*r.width+x]
			}
			if sub := y - radius; sub >= 0 {
				sum -= r.scratch[sub*r.width+x]
			}
		}
	}
}
func (r *Renderer) renderEnergy() {
	r.boxBlur(r.charge, r.bloom, max(1, r.height/100))
	for i := range r.bloom {
		r.bloom[i] = r.charge[i] + 0.35*r.bloom[i]
	}
	r.boxBlur(r.charge, r.work, max(1, r.height*45/400))
	for i := range r.bloom {
		r.bloom[i] += 0.18 * r.work[i]
	}
}
func toneIndex(e float32) int {
	if e <= 0 {
		return 0
	}
	return min(255, int(255*e/(1+e)))
}
