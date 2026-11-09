# KMS display backend

The `framebuffer/kms` package provides a DRM/KMS display backend on Linux and FreeBSD 15.1 amd64 with a compatible drm-kmod driver. It supports connector and mode selection, double-buffered scanout, and vblank-synchronized page flips. Refresh queues each page flip and waits for the previous flip before reusing its buffer, allowing callers to draw the next frame while the current flip is pending. Its pixel conversion path prepares the library's image formats for the display's scanout buffer using optimized CPU code.

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
| 90° | 5.93 ms | 24.39 ms | 4.11× |
| 180° | 21.42 ms | — | — |
| 270° | 6.27 ms | 26.11 ms | 4.16× |

The NEON path transposes 4×4 pixel tiles for 90° and 270° across CPU row bands;
edges use the portable path. It runs independent bands in parallel. These
conversion-only results allocate 474 B (7 allocations) per 90° conversion and
536 B (7 allocations) per 270° conversion on the Pi 3.
Reproduce with `go test ./internal/buffer -run '^$' -bench
'^BenchmarkCopyToXRGBRotation$' -benchmem -benchtime=1s`.

Full-resolution end-to-end measurements on the Raspberry Pi rig, using the
same 1280×400 KMS mode, 60 FPS request, and unscaled rotozoom test, with the
triple-buffered KMS path:

| Orientation | Frames in 10 s | FPS | Draw average | Refresh average |
| --- | ---: | ---: | ---: | ---: |
| 0° | 524 | 52.4 | 14.3 ms | 2.9 ms |
| 90° | 415 | 41.4 | 14.4 ms | 9.7 ms |

The rotated run reached 79.0% of the no-rotation FPS. It does not meet a
90–95% target. The rig exposes no 90°/270° fbdev rotation, and its KMS plane
advertises only 0°/180° rotation; this backend currently uses software
rotation. The KMS backend uses legacy modesetting/page flips and does not yet
set DRM plane rotation properties, so the rig cannot take a kernel 90° path.
