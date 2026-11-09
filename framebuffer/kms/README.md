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

## Rotated scanout

`Display.SetRotation` supports 90°, 180°, and 270° clockwise orientations.
Quarter turns swap the logical width and height. The current KMS backend rotates
the CPU shadow image into the native scanout buffer during `Refresh`; it does
not use a DRM plane rotation property.

RGBA-to-XRGB8888 conversion benchmarks at 800×480 on the Raspberry Pi 3
(ARMv8, Go ARM64, one second per subtest):

| Rotation | 4×4 NEON tile path | Scalar Go reference | Speedup |
| --- | ---: | ---: | ---: |
| 90° | 10.27 ms | 24.41 ms | 2.38× |
| 180° | 21.42 ms | — | — |
| 270° | 10.54 ms | 26.14 ms | 2.48× |

The NEON path transposes 4×4 pixel tiles for 90° and 270°; edges use the
portable path. These conversion-only results allocate zero bytes per frame.
Reproduce with `go test ./internal/buffer -run '^$' -bench
'^BenchmarkCopyToXRGBRotation$' -benchmem -benchtime=1s`.

Full-resolution end-to-end measurements on the Raspberry Pi rig, using the
same 1280×400 KMS mode, 60 FPS request, and unscaled rotozoom test:

| Orientation | Frames in 10 s | FPS | Draw average | Refresh average |
| --- | ---: | ---: | ---: | ---: |
| 0° | 488 | 48.8 | 14.0 ms | 6.2 ms |
| 90° | 273 | 27.2 | 14.6 ms | 22.0 ms |

The rotated run reached 55.7% of the no-rotation FPS. It does not meet a
90–95% target. The rig exposes no 90°/270° fbdev rotation, and its KMS plane
advertises only 0°/180° rotation; this backend currently uses software
rotation. The KMS backend uses legacy modesetting/page flips and does not yet
set DRM plane rotation properties, so the rig cannot take a kernel 90° path.
