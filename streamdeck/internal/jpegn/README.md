# Stream Deck JPEG encoder

This package is an encoder-only fork of [gen2brain/jpegn v0.7.1](https://github.com/gen2brain/jpegn/tree/v0.7.1),
commit `1a62ff0c9830e50b67da4472e79c377c2e11bf03`. The original MIT license is
included in [LICENSE.md](LICENSE.md).

It produces baseline JPEG with 4:2:0 chroma for color images and a single
component for `image.Gray`. The fork retains the upstream SIMD kernels, uses
only live symbols to build optimized Huffman tables, and can rotate a tile
while reading its source pixels. It writes each encoded image in one call so
the Stream Deck driver can build HID reports from the encoder's buffer.

Build with `-tags noasm` to use scalar Go kernels.
