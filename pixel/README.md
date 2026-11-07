# Pixel images

The `pixel` package provides packed monochrome, grayscale, and RGB images that
implement Go's image interfaces. The image `Fill` methods call direct assembly entry points on ARM64 and amd64.
ARM64 repeats the pattern with `VDUP` and writes 16-byte vectors with `VST1`;
amd64 uses `PUNPCKLQDQ` and 16-byte `MOVOU` stores. Other architectures use
the scalar fill loop. The optimized
path also avoids per-pixel temporary allocations when filling 16-bit images.

## Fill performance

Single-goroutine Go benchmarks for filling Gray4 and CRGB16 buffers at
400×1280. Times are the median of three 1-second benchmark runs. The scalar
column uses the original Go loops: a per-byte write for Gray4 and a repeated
two-byte copy for CRGB16.

| Test host | Format | Vector path | Vector time | Scalar time | Gain |
| --- | --- | --- | ---: | ---: | ---: |
| Raspberry Pi 3, Linux ARM64 | Gray4 | NEON | 110 µs | 1.31 ms | 11.9× |
| Raspberry Pi 3, Linux ARM64 | CRGB16 | NEON | 628 µs | 28.8 ms | 45.9× |
| Apple M2, macOS ARM64 | Gray4 | NEON | 5.58 µs | 74.4 µs | 13.3× |
| Apple M2, macOS ARM64 | CRGB16 | NEON | 22.8 µs | 1.06 ms | 46.6× |
| AMD Ryzen 9 7900, Linux x86_64 | Gray4 | SSE2 | 6.16 µs | 47.9 µs | 7.8× |
| AMD Ryzen 9 7900, Linux x86_64 | CRGB16 | SSE2 | 24.6 µs | 678 µs | 27.6× |
| Intel N100, Linux x86_64 | Gray4 | SSE2 | 10.8 µs | 110.9 µs | 10.3× |
| Intel N100, Linux x86_64 | CRGB16 | SSE2 | 43.4 µs | 1.24 ms | 28.6× |

Reproduce on a Go-equipped host with:

```sh
go test ./pixel -run '^$' -bench '^BenchmarkFill400x1280$' -benchmem -benchtime=1s -count=3
```

On the Apple M2 used for this change, repeated runs measured median optimized
fill times of 5.58 µs for Gray4 and 22.8 µs for CRGB16. Matching scalar
reference medians were 74.4 µs and 1.06 ms.

## Color conversion

The 16-bit image `Set` and `Fill` methods use typed conversion helpers so they do
not box each converted pixel into the public `color.Color` interface. Median
results for three repeated runs:

| Host | Conversion | `ColorModel.Convert` | Typed setter helper |
| --- | --- | ---: | ---: |
| Apple M2 | CRGB16 from RGBA | 11.29 ns, 1 alloc | 3.24 ns, 0 allocs |
| Apple M2 | CRGB16 from NRGBA | 10.55 ns, 1 alloc | 3.54 ns, 0 allocs |
| Raspberry Pi 3 | CRGB16 from RGBA | 402.3 ns, 1 alloc | 107.6 ns, 0 allocs |
| Raspberry Pi 3 | CRGB16 from NRGBA | 436.0 ns, 1 alloc | 140.5 ns, 0 allocs |
| AMD Ryzen 9 7900 | CRGB16 from RGBA | 9.67 ns, 1 alloc | 3.26 ns, 0 allocs |
| AMD Ryzen 9 7900 | CRGB16 from NRGBA | 9.86 ns, 1 alloc | 3.84 ns, 0 allocs |
| Intel N100 | CRGB16 from RGBA | 19.01 ns, 1 alloc | 5.99 ns, 0 allocs |
| Intel N100 | CRGB16 from NRGBA | 19.69 ns, 1 alloc | 6.97 ns, 0 allocs |

Benchmark runs used 200–500 ms per case depending on the host.

Reproduce all model/source combinations with:

```sh
go test ./pixel -run '^$' -bench '^BenchmarkPixelColorConversion$' -benchmem -benchtime=200ms -count=3
```
