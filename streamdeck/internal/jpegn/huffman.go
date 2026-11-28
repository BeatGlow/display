package jpegn

// huffEncTable maps a symbol to its canonical code and length.
type huffEncTable struct {
	code [256]uint32
	size [256]uint8
}

// buildHuffEnc builds an encoding table from DHT counts and values.
func buildHuffEnc(t *huffEncTable, bits *[17]uint8, values []byte) {
	*t = huffEncTable{}

	code := uint32(0)
	k := 0

	for l := 1; l <= 16; l++ {
		for i := 0; i < int(bits[l]); i++ {
			if k >= len(values) {
				return
			}

			sym := values[k]
			t.code[sym] = code
			t.size[sym] = uint8(l)

			code++
			k++
		}

		code <<= 1
	}
}

// maxCodeLen is the working limit before folding back to 16 bits.
const maxCodeLen = 32

// genOptimalTable builds a Huffman table from a histogram per JPEG Annex K.2.
func genOptimalTable(freq *[257]int32, bits *[17]uint8, values *[256]uint8) int {
	var f [257]int32
	copy(f[:], freq[:])
	f[256] = 1

	// Tile histograms are sparse. Keep the live symbols in index order
	// so merging still breaks frequency ties in favor of the larger index.
	var live [257]int
	active := live[:0]
	for symbol, n := range f {
		if n > 0 {
			active = append(active, symbol)
		}
	}

	var others [257]int // Linked symbols, stored as index plus one.
	var codesize [257]int

	for len(active) > 1 {
		first, second := -1, -1
		var least1, least2 int32

		for i, symbol := range active {
			if first < 0 || f[symbol] <= least1 {
				second, least2 = first, least1
				first, least1 = i, f[symbol]
			} else if second < 0 || f[symbol] <= least2 {
				second, least2 = i, f[symbol]
			}
		}

		v1, v2 := active[first], active[second]
		f[v1] += f[v2]
		copy(active[second:], active[second+1:])
		active = active[:len(active)-1]

		codesize[v1]++
		for others[v1] != 0 {
			v1 = others[v1] - 1
			codesize[v1]++
		}
		others[v1] = v2 + 1

		codesize[v2]++
		for others[v2] != 0 {
			v2 = others[v2] - 1
			codesize[v2]++
		}
	}

	var count [maxCodeLen + 1]int32
	var next [maxCodeLen + 1]int
	for i := 0; i <= 256; i++ {
		if l := codesize[i]; l > 0 {
			if l > maxCodeLen {
				l = maxCodeLen
				codesize[i] = l
			}

			count[l]++
			if i != 256 {
				next[l]++
			}
		}
	}

	for i := maxCodeLen; i > 16; i-- {
		for count[i] > 0 {
			j := i - 2
			for count[j] == 0 {
				j--
			}

			count[i] -= 2
			count[i-1]++
			count[j+1] += 2
			count[j]--
		}
	}

	i := 16
	for i > 0 && count[i] == 0 {
		i--
	}

	if i == 0 {
		return 0
	}

	count[i]--

	*bits = [17]uint8{}
	for l := 1; l <= 16; l++ {
		bits[l] = uint8(count[l])
	}

	// Assign offsets by original code length, then place symbols in
	// ascending order within each length. Length folding changes the
	// transmitted counts while keeping this symbol order.
	size := 0
	for l := 1; l <= maxCodeLen; l++ {
		count := next[l]
		next[l] = size
		size += count
	}
	for symbol, length := range codesize[:256] {
		if length > 0 {
			values[next[length]] = uint8(symbol)
			next[length]++
		}
	}

	return size
}
