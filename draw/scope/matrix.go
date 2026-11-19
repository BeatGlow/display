package scope

import "math"

type matrix struct{ A, B, C, D, E, F float64 }

var identity = matrix{A: 1, D: 1}

func (a matrix) Mult(b matrix) matrix {
	return matrix{a.A*b.A + a.C*b.B, a.B*b.A + a.D*b.B, a.A*b.C + a.C*b.D, a.B*b.C + a.D*b.D, a.A*b.E + a.C*b.F + a.E, a.B*b.E + a.D*b.F + a.F}
}
func (a matrix) Transform(x, y float64) (float64, float64) {
	return a.A*x + a.C*y + a.E, a.B*x + a.D*y + a.F
}
func (a matrix) Translate(x, y float64) matrix { return a.Mult(matrix{A: 1, D: 1, E: x, F: y}) }
func (a matrix) Scale(x, y float64) matrix     { return a.Mult(matrix{A: x, D: y}) }
func (a matrix) Rotate(v float64) matrix {
	s, c := math.Sincos(v)
	return a.Mult(matrix{A: c, B: s, C: -s, D: c})
}
func (a matrix) SkewX(v float64) matrix { return a.Mult(matrix{A: 1, C: math.Tan(v), D: 1}) }
func (a matrix) SkewY(v float64) matrix { return a.Mult(matrix{A: 1, B: math.Tan(v), D: 1}) }
