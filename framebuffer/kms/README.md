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

RGBA-to-XRGB8888 benchmarks at 800×480, with three 2-second subtest samples
on a Raspberry Pi 3 (ARMv8, Go ARM64):

| Rotation | Optimized ARM64 path | Scalar Go reference | Speedup |
| --- | ---: | ---: | ---: |
| 90° | 1.65 ms | 24.34 ms | 14.8× |
| 180° | 1.59 ms | 9.08 ms | 5.7× |
| 270° | 1.64 ms | 25.56 ms | 15.6× |

The no-rotation conversion measured 1.21 ms on the same Pi 3.

The ARM64 NEON path fuses RGBA channel packing with the rotation. It transposes
4×4 pixel tiles for 90° and 270°, and mirrors four pixels per vector for 180°;
independent row bands run in parallel. The amd64 path uses fused SSE2 4×4
transpose tiles for quarter turns and a SIMD mirror for 180°. Both have a
portable Go fallback for dimensions that do not fit their tile. On the Pi 3,
the rotated conversion paths allocate 448 B (7 allocations) per frame.

If the producer already supplies packed XRGB8888, it can skip the initial
RGBA-to-XRGB conversion. On the Pi 3, the packed-XRGB benchmark measured
1.73 ms at 90°, 1.41 ms at 180°, and 1.73 ms at 270°.
Reproduce with `go test ./internal/buffer -run '^$' -bench
'^BenchmarkCopyToXRGBRotation$' -benchmem -benchtime=2s -count=3`.

The same 800×480 RGBA benchmark on an Apple M2 measured 38.2 µs at 0°,
31.9 µs at 90°, 44.3 µs at 180°, and 31.6 µs at 270°. On an Intel N100 it
measured 77.3 µs at 0°, 66.4 µs at 90°, 62.6 µs at 180°, and 67.3 µs at 270°.
The Apple M2 results use two to three 2-second samples; the Intel N100 results
are one 1-second sample on `cida.maze.network`. These results use NEON on Apple
Silicon and SSE2 on amd64. The amd64 kernel currently uses 4×4 tiles; libyuv's
current AVX2 implementation uses larger 8×8
tiles ([source](https://chromium.googlesource.com/libyuv/libyuv/+/refs/heads/main/source/rotate_argb.cc)),
which remains a candidate for further porting and measurement.

Full-resolution end-to-end measurements on the Raspberry Pi rig, using its
preferred 400×1280 KMS mode, an unchanged 60 FPS request and unscaled rotozoom
test, with the triple-buffered KMS path (8 seconds per run):

| Orientation | Frames in 8 s | FPS | Draw average | Refresh average |
| --- | ---: | ---: | ---: | ---: |
| 0° | 406 | 50.7 | 15.1 ms | 3.2 ms |
| 90° | 354 | 44.2 | 14.9 ms | 7.6 ms |
| 180° | 404 | 50.5 | 15.1 ms | 3.7 ms |
| 270° | 356 | 44.4 | 14.7 ms | 7.8 ms |

The 90° and 270° runs reached 87.2% and 87.6% of the no-rotation FPS; 180°
reached 99.6%. Quarter turns are close to, but still below, the 90–95% target.
The rig exposes no 90°/270° fbdev rotation, and its KMS plane
advertises only 0°/180° rotation; this backend currently uses software
rotation. The KMS backend uses legacy modesetting/page flips and does not yet
set DRM plane rotation properties, so the rig cannot take a kernel 90° path.
