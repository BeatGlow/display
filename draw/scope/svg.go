// Copyright (c) 2026 beatglow
// SPDX-License-Identifier: MIT
// Ported from the BeatGlow player oscilloscope SVG compiler.
// SVG geometry is traced along its centerlines. Paint, text, and masks have
// no representation in a two-channel deflection signal.
package scope

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var svgNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?`)

type svgScanner struct{ rest string }

func (s *svgScanner) trim() { s.rest = strings.TrimLeft(s.rest, " \t\r\n,") }
func (s *svgScanner) number() (float64, error) {
	s.trim()
	n := svgNumber.FindString(s.rest)
	if n == "" {
		return 0, errors.New("expected SVG number")
	}
	s.rest = s.rest[len(n):]
	return strconv.ParseFloat(n, 64)
}

// Arc flags are single bytes. In path data both flags and the following
// coordinate can appear together without separators.
func (s *svgScanner) flag() (float64, error) {
	s.trim()
	if len(s.rest) == 0 || (s.rest[0] != '0' && s.rest[0] != '1') {
		return 0, errors.New("expected arc flag 0 or 1")
	}
	v := s.rest[0] - '0'
	s.rest = s.rest[1:]
	return float64(v), nil
}

func svgTransform(m matrix, value string) (matrix, error) {
	for value = strings.Trim(value, " \t\r\n,"); value != ""; value = strings.Trim(value, " \t\r\n,") {
		name, tail, ok := strings.Cut(value, "(")
		params, rest, closed := strings.Cut(tail, ")")
		if !ok || !closed {
			return m, errors.New("malformed SVG transform")
		}
		s := svgScanner{params}
		var a []float64
		for s.trim(); s.rest != ""; s.trim() {
			n, err := s.number()
			if err != nil {
				return m, err
			}
			a = append(a, n)
			if len(a) > 6 {
				return m, errors.New("too many transform arguments")
			}
		}
		name = strings.TrimSpace(name)
		switch {
		case name == "matrix" && len(a) == 6:
			m = m.Mult(matrix{A: a[0], B: a[1], C: a[2], D: a[3], E: a[4], F: a[5]})
		case name == "translate" && len(a) == 1:
			m = m.Translate(a[0], 0)
		case name == "translate" && len(a) == 2:
			m = m.Translate(a[0], a[1])
		case name == "scale" && len(a) == 1:
			m = m.Scale(a[0], a[0])
		case name == "scale" && len(a) == 2:
			m = m.Scale(a[0], a[1])
		case name == "rotate" && len(a) == 1:
			m = m.Rotate(a[0] * math.Pi / 180)
		case name == "rotate" && len(a) == 3:
			m = m.Translate(a[1], a[2]).Rotate(a[0]*math.Pi/180).Translate(-a[1], -a[2])
		case name == "skewX" && len(a) == 1:
			m = m.SkewX(a[0] * math.Pi / 180)
		case name == "skewY" && len(a) == 1:
			m = m.SkewY(a[0] * math.Pi / 180)
		default:
			return m, fmt.Errorf("invalid SVG transform %q", name)
		}
		value = rest
	}
	return m, nil
}

func svgShape(name string, attrs map[string]string) (string, error) {
	var keys []string
	switch name {
	case "path":
		return attrs["d"], nil
	case "polygon", "polyline":
		data := strings.TrimSpace(attrs["points"])
		if data == "" {
			return "", nil
		}
		// Only coordinates are legal in points lists.
		if strings.ContainsAny(data, "MmLlHhVvSsTtCcQqAaZz") {
			return "", errors.New("invalid points list")
		}
		if name == "polygon" {
			return "M" + data + "Z", nil
		}
		return "M" + data, nil
	case "line":
		keys = []string{"x1", "y1", "x2", "y2"}
	case "rect":
		keys = []string{"x", "y", "width", "height", "rx", "ry"}
	case "circle":
		keys = []string{"cx", "cy", "r"}
	case "ellipse":
		keys = []string{"cx", "cy", "rx", "ry"}
	default:
		return "", nil
	}
	var n [6]float64
	for i, k := range keys {
		if v, ok := attrs[k]; ok {
			f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), "px"), 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return "", fmt.Errorf("invalid %s=%q", k, v)
			}
			n[i] = f
		}
	}
	x, y := n[0], n[1]
	if name == "line" {
		return fmt.Sprintf("M%g %gL%g %g", x, y, n[2], n[3]), nil
	}
	if name == "rect" {
		w, h, rx, ry := n[2], n[3], n[4], n[5]
		if w < 0 || h < 0 || rx < 0 || ry < 0 {
			return "", errors.New("negative rectangle dimension")
		}
		if w == 0 || h == 0 {
			return "", nil
		}
		if _, ok := attrs["rx"]; !ok {
			rx = ry
		}
		if _, ok := attrs["ry"]; !ok {
			ry = rx
		}
		rx, ry = min(rx, w/2), min(ry, h/2)
		if rx == 0 || ry == 0 {
			return fmt.Sprintf("M%g %gh%gv%gh%gZ", x, y, w, h, -w), nil
		}
		corner := func(dx, dy float64) string {
			return fmt.Sprintf("a%g %g 0 0 1 %g %g", rx, ry, dx, dy)
		}
		return fmt.Sprintf("M%g %gh%g%sv%g%sh%g%sv%g%sZ", x+rx, y,
			w-2*rx, corner(rx, ry), h-2*ry, corner(-rx, ry),
			2*rx-w, corner(-rx, -ry), 2*ry-h, corner(rx, -ry)), nil
	}
	rx, ry := n[2], n[3]
	if name == "circle" {
		ry = rx
	}
	if rx < 0 || ry < 0 {
		return "", errors.New("negative ellipse radius")
	}
	if rx == 0 || ry == 0 {
		return "", nil
	}
	return fmt.Sprintf("M%g %gA%g %g 0 1 1 %g %gA%g %g 0 1 1 %g %gZ", x+rx, y, rx, ry, x-rx, y, rx, ry, x+rx, y), nil
}

type svgGeometry struct {
	paths      [][]point
	m          matrix
	resolution int
	vertices   int
}

func (g *svgGeometry) transform(p point) point { x, y := g.m.Transform(p.x, p.y); return point{x, y} }

func (g *svgGeometry) vertex(p point) error {
	if !(math.Abs(p.x) <= 1e100 && math.Abs(p.y) <= 1e100) {
		return errors.New("non-finite or excessive SVG coordinate")
	}
	path := &g.paths[len(g.paths)-1]
	if len(*path) > 0 && (*path)[len(*path)-1] == p {
		return nil
	}
	if g.vertices >= 1_000_000 {
		return errors.New("SVG exceeds one million flattened vertices")
	}
	*path = append(*path, p)
	g.vertices++
	return nil
}

func svgDistance(a, b point) float64 { return math.Hypot(a.x-b.x, a.y-b.y) }
func chordDistance(p, a, b point) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	length := dx*dx + dy*dy
	if length == 0 {
		return svgDistance(p, a)
	}
	t := min(1, max(0, ((p.x-a.x)*dx+(p.y-a.y)*dy)/length))
	return svgDistance(p, mix(a, b, t))
}

func (g *svgGeometry) cubic(a, b, c, d point) error {
	a, b, c, d = g.transform(a), g.transform(b), g.transform(c), g.transform(d)
	span := max(svgDistance(a, b), svgDistance(a, c), svgDistance(a, d))
	return g.flatten(a, b, c, d, span/(4*float64(g.resolution)), 0)
}

func (g *svgGeometry) flatten(a, b, c, d point, tol float64, depth int) error {
	if depth >= 18 || max(chordDistance(b, a, d), chordDistance(c, a, d)) <= tol {
		return g.vertex(d)
	}
	ab, bc, cd := mix(a, b, .5), mix(b, c, .5), mix(c, d, .5)
	left, right := mix(ab, bc, .5), mix(bc, cd, .5)
	mid := mix(left, right, .5)
	if err := g.flatten(a, ab, left, mid, tol, depth+1); err != nil {
		return err
	}
	return g.flatten(mid, right, cd, d, tol, depth+1)
}

// arc converts SVG endpoint parameters to the center parameterization. Each
// span of at most 45 degrees becomes a cubic, then follows the same screen-
// space flatness test as a Bézier from the source file.
func (g *svgGeometry) arc(start, end point, a [7]float64) error {
	rx, ry := math.Abs(a[0]), math.Abs(a[1])
	if start == end || rx == 0 || ry == 0 {
		return g.vertex(g.transform(end))
	}
	sin, cos := math.Sincos(a[2] * math.Pi / 180)
	dx, dy := (start.x-end.x)/2, (start.y-end.y)/2
	u, v := (cos*dx+sin*dy)/rx, (-sin*dx+cos*dy)/ry
	scale := math.Hypot(u, v)
	if scale > 1 {
		rx *= scale
		ry *= scale
		u /= scale
		v /= scale
	}
	length := u*u + v*v
	k := math.Sqrt(max(0, (1-length)/length))
	if a[3] == a[4] {
		k = -k
	}
	cx, cy := k*v, -k*u
	mid := mix(start, end, 0.5)
	project := func(x, y float64) point {
		return point{mid.x + cos*rx*(x+cx) - sin*ry*(y+cy), mid.y + sin*rx*(x+cx) + cos*ry*(y+cy)}
	}
	first := math.Atan2(v-cy, u-cx)
	delta := math.Remainder(math.Atan2(-v-cy, -u-cx)-first, 2*math.Pi)
	if a[4] == 1 && delta < 0 {
		delta += 2 * math.Pi
	}
	if a[4] == 0 && delta > 0 {
		delta -= 2 * math.Pi
	}
	if math.IsNaN(delta) {
		return errors.New("arc exceeds numeric range")
	}
	count := max(1, int(math.Ceil(math.Abs(delta)/(math.Pi/4))))
	step := delta / float64(count)
	handle := 4 * math.Tan(step/4) / 3
	for i := range count {
		s0, c0 := math.Sincos(first + float64(i)*step)
		s1, c1 := math.Sincos(first + float64(i+1)*step)
		next := project(c1, s1)
		if i == count-1 {
			next = end
		}
		if err := g.cubic(start, project(c0-handle*s0, s0+handle*c0), project(c1+handle*s1, s1-handle*c1), next); err != nil {
			return err
		}
		start = next
	}
	return nil
}

func (g *svgGeometry) path(data string) error {
	s := svgScanner{data}
	var pos, start, control point
	var cmd, prev byte
	drawing := false
	for s.trim(); s.rest != ""; s.trim() {
		c := s.rest[0]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			cmd = c
			s.rest = s.rest[1:]
		}
		upper := cmd &^ 32
		if prev == 0 && upper != 'M' {
			return errors.New("SVG path must start with moveto")
		}
		var count int
		switch upper {
		case 'M', 'L', 'T':
			count = 2
		case 'H', 'V':
			count = 1
		case 'C':
			count = 6
		case 'S', 'Q':
			count = 4
		case 'A':
			count = 7
		case 'Z':
			if drawing {
				if err := g.vertex(g.transform(start)); err != nil {
					return err
				}
			}
			pos = start
			prev = upper
			cmd = 0
			continue
		default:
			return fmt.Errorf("invalid SVG path command %q", cmd)
		}
		var a [7]float64
		for i := range count {
			var err error
			if upper == 'A' && (i == 3 || i == 4) {
				a[i], err = s.flag()
			} else {
				a[i], err = s.number()
			}
			if err != nil {
				return fmt.Errorf("command %c: %w", cmd, err)
			}
		}
		coord := func(x, y float64) point {
			if cmd >= 'a' {
				return point{pos.x + x, pos.y + y}
			}
			return point{x, y}
		}
		var next point
		var err error
		if !drawing && upper != 'M' {
			if len(g.paths) >= 8192 {
				return errors.New("too many SVG subpaths")
			}
			g.paths = append(g.paths, nil)
			if err := g.vertex(g.transform(pos)); err != nil {
				return err
			}
			drawing = true
		}
		switch upper {
		case 'M':
			next = coord(a[0], a[1])
			start = next
			drawing = false
			if cmd == 'm' {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L':
			next = coord(a[0], a[1])
			err = g.vertex(g.transform(next))
		case 'H':
			next = point{coord(a[0], 0).x, pos.y}
			err = g.vertex(g.transform(next))
		case 'V':
			next = point{pos.x, coord(0, a[0]).y}
			err = g.vertex(g.transform(next))
		case 'C', 'S':
			b := coord(a[0], a[1])
			if upper == 'S' {
				b = pos
				if prev == 'C' || prev == 'S' {
					b = mix(control, pos, 2)
				}
			}
			control = coord(a[count-4], a[count-3])
			next = coord(a[count-2], a[count-1])
			err = g.cubic(pos, b, control, next)
		case 'Q', 'T':
			b := coord(a[0], a[1])
			if upper == 'T' {
				b = pos
				if prev == 'Q' || prev == 'T' {
					b = mix(control, pos, 2)
				}
			}
			control = b
			next = coord(a[count-2], a[count-1])
			err = g.cubic(pos, mix(pos, b, 2.0/3), mix(next, b, 2.0/3), next)
		case 'A':
			next = coord(a[5], a[6])
			err = g.arc(pos, next, a)
		}
		if err != nil {
			return err
		}
		pos = next
		prev = upper
	}
	return nil
}

func compilePaths(r io.Reader, width, height int) ([][]point, error) {
	if width < 2 || height < 2 {
		return nil, errors.New("scope dimensions must each be at least two pixels")
	}
	limited := &io.LimitedReader{R: r, N: (16 << 20) + 1}
	dec := xml.NewDecoder(limited)
	stack := []matrix{identity}
	hiddenDepth := 0
	g := svgGeometry{resolution: max(width, height)}
	seenRoot := false
	for {
		token, err := dec.Token()
		if limited.N == 0 {
			return nil, errors.New("SVG exceeds 16 MiB")
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if len(stack)+hiddenDepth > 512 {
				return nil, errors.New("SVG nesting exceeds 512 levels")
			}
			if hiddenDepth > 0 {
				hiddenDepth++
				continue
			}
			if len(stack) == 1 {
				if seenRoot || t.Name.Local != "svg" {
					return nil, errors.New("expected a single svg root")
				}
				seenRoot = true
			}
			switch t.Name.Local {
			case "defs", "symbol", "metadata", "clipPath", "mask", "pattern", "marker":
				hiddenDepth = 1
				continue
			}
			attrs := make(map[string]string, len(t.Attr))
			for _, a := range t.Attr {
				attrs[a.Name.Local] = a.Value
			}
			if attrs["display"] == "none" {
				hiddenDepth = 1
				continue
			}
			m, err := svgTransform(stack[len(stack)-1], attrs["transform"])
			if err != nil {
				return nil, err
			}
			stack = append(stack, m)
			g.m = m
			data, err := svgShape(t.Name.Local, attrs)
			if err == nil {
				err = g.path(data)
			}
			if err != nil {
				return nil, fmt.Errorf("<%s>: %w", t.Name.Local, err)
			}
		case xml.EndElement:
			if hiddenDepth > 0 {
				hiddenDepth--
			} else {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if g.vertices == 0 {
		return nil, errors.New("SVG contains no traceable geometry")
	}
	low := g.paths[0][0]
	high := low
	for _, path := range g.paths {
		for _, p := range path {
			low.x = min(low.x, p.x)
			low.y = min(low.y, p.y)
			high.x = max(high.x, p.x)
			high.y = max(high.y, p.y)
		}
	}
	w, h := float64(width-1), float64(height-1)
	span := max((high.x-low.x)/w, (high.y-low.y)/h)
	if span == 0 {
		span = 1
	}
	center := mix(low, high, .5)
	for _, path := range g.paths {
		for i, p := range path {
			path[i] = point{(p.x - center.x) / span * 1.88 / w, (p.y - center.y) / span * 1.88 / h}
		}
	}
	return g.paths, nil
}
