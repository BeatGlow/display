# Drawing fast paths

`draw.DrawMask` uses a format-aware copy for unmasked `Src` operations between
matching 16-bit pixel buffers. Clipping and in-place overlapping copies are
handled; masks, `Over`, packed formats, and unsupported images use
`image/draw` directly.

On an Apple M2 (Go benchmark, 400×128 CRGB16, three runs), the local fast path
copied the 102,400-byte frame in a median 2.764 µs with zero allocations.
Compare the reference path on the target host with:

```sh
go test ./draw -run '^$' -bench '^BenchmarkBlitCRGB16$' -benchmem -count=3
```
