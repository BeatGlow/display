# KMS display backend

The `framebuffer/kms` package provides a DRM/KMS display backend on Linux and FreeBSD 15.1 amd64 with a compatible drm-kmod driver. It supports connector and mode selection, double-buffered scanout, and vblank-synchronized page flips. Its pixel conversion path prepares the library's image formats for the display's scanout buffer using optimized CPU code.

## Pixel conversion performance

Single-goroutine Go benchmarks for RGBA-to-XRGB8888 conversion at 400×1280:

| Test system | Optimized path | Scalar path | Gain |
| --- | ---: | ---: | ---: |
| Raspberry Pi 3 | 2.318 ms | 19.202 ms | 8.3× |
| Apple M2 | 56.6 µs | 841.9 µs | 14.9× |
| AMD Ryzen 9 7900 | 58.6 µs | 704.1 µs | 12.0× |
| Intel N100 | 125.8 µs | 1.516 ms | 12.0× |

The KMS demonstration also sustained about 51 FPS on a Raspberry Pi 3 at 400×1280 with a 60 FPS target. This end-to-end measurement includes rendering, scanout conversion, and synchronized page flips.
