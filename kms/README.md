# KMS display backend

The `kms` package provides a Linux DRM/KMS display backend with connector and mode selection, double-buffered scanout, and vblank-synchronized page flips. Its pixel conversion path prepares the library's image formats for the display's scanout buffer using optimized CPU code.

## Pixel conversion performance

Single-goroutine Go benchmarks for RGBA-to-XRGB8888 conversion at 400×1280:

| Test system | Optimized path | Scalar path | Gain |
| --- | ---: | ---: | ---: |
| Raspberry Pi 3 | 2.61 ms | 19.23 ms | 7.4× |
| Apple M2 | 60.4 µs | 991 µs | 16.4× |
| AMD Ryzen 9 7900 | 59.9 µs | 712 µs | 11.9× |
| Intel N100 | 131 µs | 1.52 ms | 11.6× |

The KMS demonstration also sustained about 51 FPS on a Raspberry Pi 3 at 400×1280 with a 60 FPS target. This end-to-end measurement includes rendering, scanout conversion, and synchronized page flips.
