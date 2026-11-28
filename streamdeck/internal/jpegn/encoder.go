// Package jpegn encodes baseline JPEG images for Stream Deck keys and strips.
package jpegn

import (
	"errors"
	"image"
	"io"
	"math/bits"
	"sync"
)

// JPEG markers emitted by the encoder.
const (
	markerSOI  = 0xD8
	markerEOI  = 0xD9
	markerSOF0 = 0xC0
	markerDHT  = 0xC4
	markerDQT  = 0xDB
	markerSOS  = 0xDA
	markerAPP0 = 0xE0
)

// DefaultQuality is selected when Options.Quality is zero.
const DefaultQuality = 75

// invZz maps a natural coefficient index to its zigzag position.
var invZz [64]uint8

func init() {
	for k, n := range zz {
		invZz[n] = uint8(k)
	}
}

// quantShift is the fixed-point scale of the quantization reciprocals.
const quantShift = 31

// Options selects compression and orientation for a baseline JPEG.
type Options struct {
	Quality        int  // 1 through 100, or zero for the default.
	OptimizeCoding bool // Derive Huffman tables from the image.
	Rotate180      bool // Rotate the input before encoding.
}

// Quantization tables from the JPEG standard, Annex K.1, in natural order.
var stdLumaQuant = [64]uint16{
	16, 11, 10, 16, 24, 40, 51, 61,
	12, 12, 14, 19, 26, 58, 60, 55,
	14, 13, 16, 24, 40, 57, 69, 56,
	14, 17, 22, 29, 51, 87, 80, 62,
	18, 22, 37, 56, 68, 109, 103, 77,
	24, 35, 55, 64, 81, 104, 113, 92,
	49, 64, 78, 87, 103, 121, 120, 101,
	72, 92, 95, 98, 112, 100, 103, 99,
}

var stdChromaQuant = [64]uint16{
	17, 18, 24, 47, 99, 99, 99, 99,
	18, 21, 26, 66, 99, 99, 99, 99,
	24, 26, 56, 99, 99, 99, 99, 99,
	47, 66, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
}

// encComponent is one color component of the encoder.
type encComponent struct {
	id           int
	ssX, ssY     int
	qtSel        int
	dcSel, acSel int
	pred         int32
	plane        []byte
	stride       int
	width        int
	height       int

	hf, vf   int
	coeffs   []int16
	masks    []uint64
	nBlocksX int
}

// encoder holds the state of the JPEG encoding process.
type encoder struct {
	out           []byte
	acc           uint64
	nacc          uint
	width, height int
	ncomp         int
	comp          [3]encComponent
	hmax, vmax    int
	mcusX, mcusY  int
	qtab          [2][64]uint16 // Zigzag order, as written to DQT.
	qrecip        [2][64]int32  // Natural order, reciprocal of the FDCT-matched divisor.
	qhalf         [2][64]int32  // Natural order, half the divisor, for rounding.
	nqtab         int
	dcTab         [2]huffEncTable
	acTab         [2]huffEncTable
	dcBits        [2][17]uint8
	acBits        [2][17]uint8
	dcVals        [2][256]uint8
	acVals        [2][256]uint8
	dcCount       [2]int
	acCount       [2]int
	nhuff         int
	dcFreq        [2][257]int32
	acFreq        [2][257]int32
	gather        bool
	buffered      bool
	rotate180     bool
	blk           [64]int32
	zblk          [64]int32
	rowBuf        []byte
	chromaBuf     []byte
}

// encoderPool is a pool of encoder structs to reduce allocation overhead.
var encoderPool = sync.Pool{
	New: func() interface{} {
		return &encoder{}
	},
}

// Encode writes a baseline JPEG in one call to w.Write. Its buffer is reused
// after Write returns. A quality of zero selects DefaultQuality.
func Encode(w io.Writer, m image.Image, opts Options) error {
	b := m.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || b.Dx() > 65535 || b.Dy() > 65535 {
		return errors.New("jpegn: invalid image dimensions")
	}
	if opts.Quality == 0 {
		opts.Quality = DefaultQuality
	}

	e := encoderPool.Get().(*encoder)
	defer func() {
		e.out = e.out[:0]
		encoderPool.Put(e)
	}()

	e.encode(m, opts)
	n, err := w.Write(e.out)
	if err == nil && n != len(e.out) {
		return io.ErrShortWrite
	}
	return err
}

// encode runs the baseline compression pipeline into e.out.
func (e *encoder) encode(m image.Image, opts Options) {
	b := m.Bounds()
	e.acc = 0
	e.nacc = 0
	e.width = b.Dx()
	e.height = b.Dy()
	e.rotate180 = opts.Rotate180
	e.buffered = opts.OptimizeCoding

	_, gray := m.(*image.Gray)
	e.setSampling(gray)
	e.mcusX = (e.width + e.hmax*8 - 1) / (e.hmax * 8)
	e.mcusY = (e.height + e.vmax*8 - 1) / (e.vmax * 8)
	e.buildQuant(opts.Quality)
	e.buildPlanes(m)

	if e.buffered {
		e.buildCoeffs()
		e.gather = true
		e.dcFreq = [2][257]int32{}
		e.acFreq = [2][257]int32{}
		e.scan()
		e.gather = false
		e.buildOptimalTables()
	} else {
		e.setStdTables()
	}

	e.writeHeader()
	e.scan()
	e.emitMarker(markerEOI)
}

// setComp configures one component, keeping its allocated plane.
func (e *encoder) setComp(i, id, ssX, ssY, sel int) {
	c := &e.comp[i]
	c.id = id
	c.ssX, c.ssY = ssX, ssY
	c.hf, c.vf = e.hmax/ssX, e.vmax/ssY
	c.qtSel, c.dcSel, c.acSel = sel, sel, sel
	c.pred = 0
}

// setSampling configures the component list for a subsampling ratio.
func (e *encoder) setSampling(gray bool) {
	if gray {
		e.ncomp = 1
		e.nqtab = 1
		e.nhuff = 1
		e.hmax, e.vmax = 1, 1
		e.setComp(0, 1, 1, 1, 0)

		return
	}

	e.hmax, e.vmax = 2, 2

	e.ncomp = 3
	e.nqtab = 2
	e.nhuff = 2
	e.setComp(0, 1, e.hmax, e.vmax, 0)
	e.setComp(1, 2, 1, 1, 1)
	e.setComp(2, 3, 1, 1, 1)
}

// buildQuant scales the standard tables for a quality into zigzag order.
func (e *encoder) buildQuant(quality int) {
	if quality < 1 {
		quality = 1
	}

	if quality > 100 {
		quality = 100
	}

	scale := 200 - quality*2
	if quality < 50 {
		scale = 5000 / quality
	}

	for t := 0; t < e.nqtab; t++ {
		base := &stdLumaQuant
		if t == 1 {
			base = &stdChromaQuant
		}

		for i := 0; i < 64; i++ {
			nat := zz[i]

			v := (int(base[nat])*scale + 50) / 100
			if v < 1 {
				v = 1
			}

			if v > 255 {
				v = 255
			}

			d := int32(v) * 8
			e.qtab[t][i] = uint16(v)
			e.qhalf[t][nat] = d >> 1
			e.qrecip[t][nat] = int32((1<<quantShift + int64(d) - 1) / int64(d))
		}
	}
}

// setStdTables installs the Huffman tables from JPEG Annex K.
func (e *encoder) setStdTables() {
	for t := 0; t < e.nhuff; t++ {
		dcCounts, dcValues := &defaultDCLumaCounts, defaultDCLumaValues
		acCounts, acValues := &defaultACLumaCounts, defaultACLumaValues

		if t == 1 {
			dcCounts, dcValues = &defaultDCChromaCounts, defaultDCChromaValues
			acCounts, acValues = &defaultACChromaCounts, defaultACChromaValues
		}

		e.dcBits[t] = [17]uint8{}
		e.acBits[t] = [17]uint8{}

		for l := 1; l <= 16; l++ {
			e.dcBits[t][l] = dcCounts[l-1]
			e.acBits[t][l] = acCounts[l-1]
		}

		e.dcCount[t] = copy(e.dcVals[t][:], dcValues)
		e.acCount[t] = copy(e.acVals[t][:], acValues)

		buildHuffEnc(&e.dcTab[t], &e.dcBits[t], e.dcVals[t][:e.dcCount[t]])
		buildHuffEnc(&e.acTab[t], &e.acBits[t], e.acVals[t][:e.acCount[t]])
	}
}

// buildOptimalTables derives Huffman tables from the gathered statistics.
func (e *encoder) buildOptimalTables() {
	e.setStdTables()

	for t := 0; t < e.nhuff; t++ {
		var dcBits [17]uint8
		var dcVals [256]uint8

		if n := genOptimalTable(&e.dcFreq[t], &dcBits, &dcVals); n > 0 {
			e.dcBits[t] = dcBits
			e.dcVals[t] = dcVals
			e.dcCount[t] = n
			buildHuffEnc(&e.dcTab[t], &e.dcBits[t], e.dcVals[t][:n])
		}

		var acBits [17]uint8
		var acVals [256]uint8

		if n := genOptimalTable(&e.acFreq[t], &acBits, &acVals); n > 0 {
			e.acBits[t] = acBits
			e.acVals[t] = acVals
			e.acCount[t] = n
			buildHuffEnc(&e.acTab[t], &e.acBits[t], e.acVals[t][:n])
		}
	}
}

// emitMarker writes a standalone marker.
func (e *encoder) emitMarker(m byte) {
	e.out = append(e.out, 0xFF, m)
}

// emitU16 writes a big-endian 16-bit value.
func (e *encoder) emitU16(v int) {
	e.out = append(e.out, byte(v>>8), byte(v))
}

// emitDHT writes one Huffman table segment.
func (e *encoder) emitDHT(class, id int, tbits *[17]uint8, values []byte) {
	e.emitMarker(markerDHT)
	e.emitU16(2 + 1 + 16 + len(values))
	e.out = append(e.out, byte(class<<4|id))
	e.out = append(e.out, tbits[1:17]...)
	e.out = append(e.out, values...)
}

// writeHeader emits everything from SOI up to and including SOS.
func (e *encoder) writeHeader() {
	e.writeFrameHeader()

	for t := 0; t < e.nhuff; t++ {
		e.emitDHT(0, t, &e.dcBits[t], e.dcVals[t][:e.dcCount[t]])
		e.emitDHT(1, t, &e.acBits[t], e.acVals[t][:e.acCount[t]])
	}

	var comps [4]int
	for i := 0; i < e.ncomp; i++ {
		comps[i] = i
	}

	e.writeSOS(&comps, e.ncomp, 0, 63, 0, 0)
}

// writeFrameHeader emits the JFIF header, quantization tables and baseline frame header.
func (e *encoder) writeFrameHeader() {
	e.emitMarker(markerSOI)
	e.emitMarker(markerAPP0)
	e.emitU16(16)
	e.out = append(e.out, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0)

	for t := 0; t < e.nqtab; t++ {
		e.emitMarker(markerDQT)
		e.emitU16(2 + 1 + 64)
		e.out = append(e.out, byte(t))

		for i := 0; i < 64; i++ {
			e.out = append(e.out, byte(e.qtab[t][i]))
		}
	}

	e.emitMarker(markerSOF0)
	e.emitU16(8 + 3*e.ncomp)
	e.out = append(e.out, 8)
	e.emitU16(e.height)
	e.emitU16(e.width)
	e.out = append(e.out, byte(e.ncomp))

	for i := 0; i < e.ncomp; i++ {
		c := &e.comp[i]
		e.out = append(e.out, byte(c.id), byte(c.ssX<<4|c.ssY), byte(c.qtSel))
	}
}

// writeSOS emits a scan header over the given components and spectral range.
func (e *encoder) writeSOS(comps *[4]int, ncomp, ss, se, ah, al int) {
	e.emitMarker(markerSOS)
	e.emitU16(6 + 2*ncomp)
	e.out = append(e.out, byte(ncomp))

	for i := 0; i < ncomp; i++ {
		c := &e.comp[comps[i]]
		e.out = append(e.out, byte(c.id), byte(c.dcSel<<4|c.acSel))
	}

	e.out = append(e.out, byte(ss), byte(se), byte(ah<<4|al))
}

// emitBits appends nbits of code, applying JPEG byte stuffing.
func (e *encoder) emitBits(code uint32, nbits uint8) {
	e.acc = e.acc<<nbits | uint64(code)
	e.nacc += uint(nbits)

	if e.nacc < 32 {
		return
	}

	e.nacc -= 32
	v := uint32(e.acc >> e.nacc)

	// A set bit marks a 0xFF byte, which needs a stuffed zero after it.
	if (^v-0x01010101)&v&0x80808080 != 0 {
		e.stuff(v)

		return
	}

	e.out = append(e.out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// emitPair appends a Huffman symbol and its value bits in one accumulator step.
// Both sizes together never exceed 32, the room the accumulator always has.
func (e *encoder) emitPair(code uint32, nbits uint8, val uint32, vbits uint8) {
	n := nbits + vbits
	e.acc = e.acc<<n | uint64(code)<<vbits | uint64(val)
	e.nacc += uint(n)

	if e.nacc < 32 {
		return
	}

	e.nacc -= 32
	v := uint32(e.acc >> e.nacc)

	if (^v-0x01010101)&v&0x80808080 != 0 {
		e.stuff(v)

		return
	}

	e.out = append(e.out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// stuff writes four bytes that contain at least one 0xFF.
func (e *encoder) stuff(v uint32) {
	for s := 24; s >= 0; s -= 8 {
		b := byte(v >> uint(s))
		e.out = append(e.out, b)

		if b == 0xFF {
			e.out = append(e.out, 0x00)
		}
	}
}

// flushBits pads to a byte boundary with one bits.
func (e *encoder) flushBits() {
	for e.nacc >= 8 {
		e.nacc -= 8

		b := byte(e.acc >> e.nacc)
		e.out = append(e.out, b)

		if b == 0xFF {
			e.out = append(e.out, 0x00)
		}
	}

	if e.nacc > 0 {
		pad := 8 - e.nacc

		b := byte(e.acc<<pad) | byte(1<<pad-1)
		e.out = append(e.out, b)

		if b == 0xFF {
			e.out = append(e.out, 0x00)
		}
	}

	e.acc = 0
	e.nacc = 0
}

// magnitude returns the JPEG size category of v and the coefficient bits.
func magnitude(v int32) (uint8, uint32) {
	m := v >> 31
	n := bits.Len32(uint32((v ^ m) - m))

	return uint8(n), uint32(v + (1<<uint(n)-1)&m)
}

// scan walks the MCUs, gathering statistics or emitting coded data.
func (e *encoder) scan() {
	for i := 0; i < e.ncomp; i++ {
		e.comp[i].pred = 0
	}

	for my := 0; my < e.mcusY; my++ {
		for mx := 0; mx < e.mcusX; mx++ {
			for ci := 0; ci < e.ncomp; ci++ {
				c := &e.comp[ci]

				for by := 0; by < c.ssY; by++ {
					for bx := 0; bx < c.ssX; bx++ {
						e.encodeBlock(ci, mx*c.ssX+bx, my*c.ssY+by)
					}
				}
			}
		}
	}

	if !e.gather {
		e.flushBits()
	}
}

// encodeBlock transforms, quantizes and codes the block at (bx, by), or codes it from c.coeffs when buffered.
func (e *encoder) encodeBlock(ci, bx, by int) {
	c := &e.comp[ci]

	if e.buffered {
		e.encodeBuffered(c, by*c.nBlocksX+bx)

		return
	}

	blk := &e.blk
	zb := &e.zblk

	fdct(blk, c.plane[by*8*c.stride+bx*8:], c.stride)

	nz := quantizeBlock(zb, blk, &e.qrecip[c.qtSel], &e.qhalf[c.qtSel])

	e.encodeCoeffs(zb, c, nz)
}

// encodeCoeffs codes one block of quantized coefficients. nz marks the non-zero
// natural positions; it is permuted to zigzag order so only those are visited.
func (e *encoder) encodeCoeffs(zb *[64]int32, c *encComponent, nz uint64) {
	diff := zb[0] - c.pred
	c.pred = zb[0]

	s, b := magnitude(diff)

	if e.gather {
		e.dcFreq[c.dcSel][s]++
	} else {
		t := &e.dcTab[c.dcSel]
		e.emitPair(t.code[s], t.size[s], b, s)
	}

	af := &e.acFreq[c.acSel]
	t := &e.acTab[c.acSel]

	zm := uint64(0)

	for m := nz &^ 1; m != 0; m &= m - 1 {
		zm |= 1 << invZz[bits.TrailingZeros64(m)]
	}

	prev := 0

	for zm != 0 {
		k := bits.TrailingZeros64(zm)
		zm &= zm - 1

		run := k - prev - 1
		for run > 15 {
			if e.gather {
				af[0xF0]++
			} else {
				e.emitBits(t.code[0xF0], t.size[0xF0])
			}

			run -= 16
		}

		s, b := magnitude(zb[zz[k]])
		sym := run<<4 | int(s)

		if e.gather {
			af[sym]++
		} else {
			e.emitPair(t.code[sym], t.size[sym], b, s)
		}

		prev = k
	}

	if prev < 63 {
		if e.gather {
			af[0]++
		} else {
			e.emitBits(t.code[0], t.size[0])
		}
	}
}

// encodeBuffered codes block blk from the zigzag coefficients buildCoeffs kept.
func (e *encoder) encodeBuffered(c *encComponent, blk int) {
	coefs := (*[64]int16)(c.coeffs[blk*64:])

	dc := int32(coefs[0])
	diff := dc - c.pred
	c.pred = dc

	s, b := magnitude(diff)

	if e.gather {
		e.dcFreq[c.dcSel][s]++
	} else {
		t := &e.dcTab[c.dcSel]
		e.emitPair(t.code[s], t.size[s], b, s)
	}

	af := &e.acFreq[c.acSel]
	t := &e.acTab[c.acSel]

	prev := 0

	for zm := c.masks[blk] &^ 1; zm != 0; zm &= zm - 1 {
		k := bits.TrailingZeros64(zm)

		run := k - prev - 1
		for run > 15 {
			if e.gather {
				af[0xF0]++
			} else {
				e.emitBits(t.code[0xF0], t.size[0xF0])
			}

			run -= 16
		}

		s, b := magnitude(int32(coefs[k]))
		sym := run<<4 | int(s)

		if e.gather {
			af[sym]++
		} else {
			e.emitPair(t.code[sym], t.size[sym], b, s)
		}

		prev = k
	}

	if prev < 63 {
		if e.gather {
			af[0]++
		} else {
			e.emitBits(t.code[0], t.size[0])
		}
	}
}

// buildCoeffs transforms and quantizes every block into zigzag order.
func (e *encoder) buildCoeffs() {
	for ci := 0; ci < e.ncomp; ci++ {
		c := &e.comp[ci]
		c.nBlocksX = e.mcusX * c.ssX
		nBlocksY := e.mcusY * c.ssY
		blocks := c.nBlocksX * nBlocksY

		if need := blocks * 64; cap(c.coeffs) < need {
			c.coeffs = make([]int16, need)
		} else {
			c.coeffs = c.coeffs[:need]
		}

		if cap(c.masks) < blocks {
			c.masks = make([]uint64, blocks)
		} else {
			c.masks = c.masks[:blocks]
		}

		recip, half := &e.qrecip[c.qtSel], &e.qhalf[c.qtSel]
		for by := 0; by < nBlocksY; by++ {
			for bx := 0; bx < c.nBlocksX; bx++ {
				fdct(&e.blk, c.plane[by*8*c.stride+bx*8:], c.stride)
				nz := quantizeBlock(&e.zblk, &e.blk, recip, half)

				blk := by*c.nBlocksX + bx
				dst := (*[64]int16)(c.coeffs[blk*64:])
				dst[0] = int16(e.zblk[0])

				var mask uint64
				for t := nz; t != 0; t &= t - 1 {
					n := bits.TrailingZeros64(t)
					k := invZz[n]
					dst[k&63] = int16(e.zblk[n])
					mask |= 1 << k
				}
				c.masks[blk] = mask
			}
		}
	}
}
