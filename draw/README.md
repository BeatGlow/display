# Drawing fast paths

`draw.DrawMask` uses format-aware paths for unmasked `Src` operations into
16-bit pixel buffers: matching-format copies (including overlap-safe copies),
solid `image.Uniform` spans, and `*image.RGBA` sources. Other formats, masks,
and Porter-Duff composition use the standard library path.

On an Apple M2, three 1-second benchmark runs for 400×128 CRGB16 measured:

| Operation | Fast median | `image/draw` median | Speedup |
| --- | ---: | ---: | ---: |
| Same-format blit | 2.70 µs | 474.9 µs | 176× |
| Uniform span | 1.89 µs | 617.5 µs | 327× |
| RGBA source conversion | 187.8 µs | 893.3 µs | 4.8× |

The optimized blit allocates nothing. Uniform spans allocate 4 B in two small
allocations. These measurements cover a 102,400-byte destination and do not
include display refresh or hardware transfer time.

Reproduce on the target host with:

```sh
go test ./draw -run '^$' -bench 'Benchmark(BlitCRGB16|UniformSpanCRGB16|RGBAtoCRGB16)$' -benchmem -benchtime=1s -count=3
```
