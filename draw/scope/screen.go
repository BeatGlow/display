package scope

import "math"

// initScreen precomputes the face and etched intersections in output space.
// The remap already holds the matching curvature for the electron trace.
func (r *Renderer) initScreen() {
	r.base = make([]float32, r.width*r.height)
	if !r.cfg.Graticule {
		return
	}
	hw, hh := float64(r.width-1)/2, float64(r.height-1)/2
	for i, uv := range r.remap {
		if uv.index[0] < 0 {
			continue
		}
		var sx, sy float64
		for j, index := range uv.index {
			sx += float64(int(index)%r.width) * float64(uv.weight[j])
			sy += float64(int(index)/r.width) * float64(uv.weight[j])
		}
		x, y := sx-hw, sy-hh
		gx := math.Abs(math.Remainder(x, hw/5))
		gy := math.Abs(math.Remainder(y, hh/4))
		if r.height < 128 {
			if gx < 0.6 && gy < 0.6 {
				r.base[i] = 0.008
			}
		} else if gx < 0.5 || gy < 0.5 {
			r.base[i] = 0.008
		}
	}
}
