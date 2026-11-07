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
| AMD Ryzen 9 7900, Linux x86_64 | Gray4 | SSE2 | 6.16 µs | 48.2 µs | 7.8× |
| AMD Ryzen 9 7900, Linux x86_64 | CRGB16 | SSE2 | 25.2 µs | 602 µs | 23.9× |
| Intel N100, Linux x86_64 | Gray4 | SSE2 | 10.9 µs | 113 µs | 10.4× |
| Intel N100, Linux x86_64 | CRGB16 | SSE2 | 47.6 µs | 1.24 ms | 26.0× |

Reproduce on a Go-equipped host with:

```sh
go test ./pixel -run '^$' -bench '^BenchmarkFill400x1280$' -benchmem -benchtime=1s -count=3
```

On the Apple M2 used for this change, repeated runs measured median optimized
fill times of 5.64 µs for Gray4 and 21.70 µs for CRGB16. The matching scalar
reference medians were 76.97 µs and 1.11 ms.
