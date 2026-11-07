# Pixel images

The `pixel` package provides packed monochrome, grayscale, and RGB images that
implement Go's image interfaces. The image `Fill` methods use NEON on ARM64 and
SSE2 on amd64, with a scalar fallback for other architectures. The optimized
path also avoids per-pixel temporary allocations when filling 16-bit images.

## Fill performance

Single-goroutine Go benchmarks for filling Gray4 and CRGB16 buffers at
400×1280. Times are the median of three 1-second benchmark runs. The scalar
column uses the original Go loops: a per-byte write for Gray4 and a repeated
two-byte copy for CRGB16.

| Test host | Format | Optimized path | SIMD time | Scalar time | Gain |
| --- | --- | --- | ---: | ---: | ---: |
| Raspberry Pi 3, Linux ARM64 | Gray4 | NEON | 110 µs | 1.31 ms | 11.9× |
| Raspberry Pi 3, Linux ARM64 | CRGB16 | NEON | 628 µs | 28.8 ms | 45.9× |
| Apple M2, macOS ARM64 | Gray4 | NEON | 5.43 µs | 75.9 µs | 14.0× |
| Apple M2, macOS ARM64 | CRGB16 | NEON | 21.7 µs | 1.09 ms | 50.1× |
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
