# Drawing fast paths

`draw.DrawMask` uses format-aware paths for unmasked `Src` operations into
16-bit pixel buffers: matching-format copies (including overlap-safe copies),
solid `image.Uniform` spans, and `*image.RGBA` sources. Other formats, masks,
and Porter-Duff composition use the standard library path.

Three repeated Go benchmark runs for 400×128 CRGB16 measured:

| Host | Operation | Fast median | `image/draw` median | Speedup |
| --- | --- | ---: | ---: | ---: |
| Raspberry Pi 3 | Same-format blit | 57.4 µs | 15.66 ms | 273× |
| Raspberry Pi 3 | Uniform span | 76.1 µs | 11.99 ms | 158× |
| Raspberry Pi 3 | RGBA source conversion | 6.46 ms | 30.14 ms | 4.7× |
| Apple M2 | Same-format blit | 2.70 µs | 474.9 µs | 176× |
| Apple M2 | Uniform span | 1.89 µs | 617.5 µs | 327× |
| Apple M2 | RGBA source conversion | 187.8 µs | 893.3 µs | 4.8× |
| AMD Ryzen 9 7900 | Same-format blit | 1.57 µs | 400.9 µs | 255× |
| AMD Ryzen 9 7900 | Uniform span | 3.41 µs | 313.4 µs | 92× |
| AMD Ryzen 9 7900 | RGBA source conversion | 159.3 µs | 692.6 µs | 4.3× |
| Intel N100 | Same-format blit | 4.20 µs | 790.3 µs | 188× |
| Intel N100 | Uniform span | 7.39 µs | 638.9 µs | 86× |
| Intel N100 | RGBA source conversion | 341.9 µs | 1.53 ms | 4.5× |

The optimized blit allocates nothing. Uniform spans allocate 4 B in two small
allocations. These measurements cover a 102,400-byte destination and do not
include display refresh or hardware transfer time.

Reproduce on the target host with:

```sh
go test ./draw -run '^$' -bench 'Benchmark(BlitCRGB16|UniformSpanCRGB16|RGBAtoCRGB16)$' -benchmem -benchtime=1s -count=3
```
