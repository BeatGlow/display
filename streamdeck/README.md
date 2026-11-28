# Stream Deck

The package controls the following Elgato Stream Deck models on Linux and
macOS. It uses one USB HID connection for input and output and builds with
`CGO_ENABLED=0` on both platforms.

| Model | USB product ID | Keys | Key image | Protocol reference | Typical full-deck FPS (Q75 / Q100) | Dense full-deck FPS (Q75 / Q100) |
| --- | --- | --- | --- | --- | --- | --- |
| Stream Deck Original (2017) | `0x0060` | 5 × 3 | 72 × 72 BMP | [Original driver reference](https://github.com/abcminiuser/python-elgato-streamdeck/blob/master/src/StreamDeck/Devices/StreamDeckOriginal.py) | Unmeasured | Unmeasured |
| Stream Deck Mini | `0x0063` | 3 × 2 | 80 × 80 BMP | [Elgato Mini](https://docs.elgato.com/streamdeck/hid/mini/) | Unmeasured | Unmeasured |
| Stream Deck XL | `0x006c` | 8 × 4 | 96 × 96 JPEG | [Elgato XL](https://docs.elgato.com/streamdeck/hid/stream-deck-xl/) | Unmeasured | Unmeasured |
| Stream Deck 2019 | `0x006d` | 5 × 3 | 72 × 72 JPEG | [Elgato Classic](https://docs.elgato.com/streamdeck/hid/stream-deck-classic/) | Unmeasured | Unmeasured |
| Stream Deck MK.2 | `0x0080` | 5 × 3 | 72 × 72 JPEG | [Elgato Classic](https://docs.elgato.com/streamdeck/hid/stream-deck-classic/) | Unmeasured | Unmeasured |
| Stream Deck + | `0x0084` | 4 × 2 | 120 × 120 JPEG | [Elgato Plus](https://docs.elgato.com/streamdeck/hid/stream-deck-plus/) | 23.97 / 20.40 | 16.31 / 7.75 |
| Stream Deck Mini 2022 | `0x0090` | 3 × 2 | 80 × 80 BMP | [Elgato Mini](https://docs.elgato.com/streamdeck/hid/mini/) | Unmeasured | Unmeasured |

The FPS columns report host-side full-deck upload rates. The quality settings
apply to JPEG. BMP models use a single encoding mode. See the
[benchmark method and per-workload rates](#upload-fps).

Elgato's Classic documentation excludes the 2017 device. Its legacy two-page
BMP transfer and right-to-left wire key order follow the linked Original
driver reference. The Mini follows Elgato's legacy report layout, with the
BMP scanline-axis correction used by the [Mini reference
implementation](https://github.com/abcminiuser/python-elgato-streamdeck/blob/master/src/StreamDeck/Devices/StreamDeckMini.py)
and its [image conversion helper](https://github.com/abcminiuser/python-elgato-streamdeck/blob/master/src/StreamDeck/ImageHelpers/PILHelper.py).

Call `streamdeck.Devices()` to list supported devices and their model
capabilities. Pass a listed path to `streamdeck.Open`, or use `Open("")` when only
one supported device is attached. On Linux, the process needs read and write
access to the device's `/dev/hidraw*` node. On macOS, the process needs permission
to open the device through IOKit.

Draw key images into standard Go images sized using `deck.Info().Model.KeySize`
and send them with `SetKeyImage`. The driver handles orientation, encoding, and
transfer. Keys are numbered from zero, left to right and then top to bottom.

The Stream Deck + also exposes four push-button dials and an 800 × 100 touch
strip. Dials are numbered from zero, left to right. Send an image sized to
`deck.Info().Model.StripSize` with `SetStripImage`.

Use `ReadEvent(ctx)` for key and dial presses and releases, signed dial rotation,
and touchscreen gestures. Touch coordinates are in strip pixels. `ReadEvent`
can run while images are being sent and can resume after its context is
canceled. `Close` interrupts pending reads and releases the HID connection.

## Upload FPS

The driver uses an [internal fork of the jpegn encoder](internal/jpegn/README.md).
Baseline JPEG quality 75 is the driver default. Quality 100 was measured in a
temporary benchmark build.

### Encoder benchmark

Median encode time for control graphics at quality 75 on an Apple M2 with
`CGO_ENABLED=0`. The 72 × 72 and 96 × 96 cases include the 180° rotation used
by the corresponding Deck models.

| Tile | Go `image/jpeg` | Upstream `jpegn` with optimized Huffman | Internal fork |
| --- | ---: | ---: | ---: |
| 72 × 72 | 150 µs | 143 µs | 43 µs |
| 96 × 96 | 216 µs | 211 µs | 60 µs |
| 120 × 120 | 191 µs | 92 µs | 71 µs |
| 800 × 100 | 1,131 µs | 350 µs | 349 µs |

With the scalar build, the fork took 153 µs for a 120 × 120 control key and
882 µs for the strip. The NEON-enabled build took 71 µs and 349 µs respectively.
The upstream comparison used v0.7.1. Across the reference inputs, the fork
produced identical JPEG bytes at qualities 75 and 100 for the same Huffman
setting.

Run the Go-versus-fork benchmark from the repository root.

```sh
CGO_ENABLED=0 go test ./streamdeck/internal/jpegn -run '^$' -bench '^BenchmarkTiles$' \
  -benchmem -count=10 -benchtime=40ms -cpu=1
```

Add `-tags noasm` to measure the scalar build.

### Physical Stream Deck + benchmark

Measured on Apple Silicon macOS with Go 1.27.3 and `CGO_ENABLED=0`. Each full
update sends eight keys and the 800 × 100 strip sequentially. The stock driver
and encoder variants ran in balanced order with the same pre-rendered RGBA
inputs, using 40 samples per individual key or strip workload and 32 per
full-deck workload. One failed HID write block was retried.

Upload FPS is completed updates divided by the sum of their timed durations.
Arrows show the stock Go encoder and packet writer followed by the internal
fork and reusable packet writer. Timings cover JPEG encoding and HID writes.
LCD refresh remains unmeasured.

| Workload | Quality | Median (ms) | p95 (ms) | Upload FPS | HID reports |
| --- | ---: | ---: | ---: | ---: | ---: |
| Controls key | 75 | 1.38 → 1.24 | 2.03 → 1.35 | 671.80 → 825.96 | 3 → 3 |
| Controls strip | 75 | 7.94 → 4.51 | 81.71 → 93.27 | 66.49 → 71.83 | 13 → 11 |
| Controls full deck | 75 | 24.43 → 20.42 | 101.97 → 96.63 | 24.26 → 23.97 | 37 → 35 |
| Controls key | 100 | 4.27 → 3.75 | 6.04 → 4.39 | 221.78 → 265.34 | 8 → 7 |
| Controls strip | 100 | 16.21 → 12.23 | 36.89 → 55.00 | 50.78 → 48.57 | 39 → 31 |
| Controls full deck | 100 | 54.10 → 48.29 | 72.00 → 67.47 | 18.62 → 20.40 | 103 → 87 |
| Dense key | 75 | 5.06 → 4.40 | 7.50 → 5.86 | 190.31 → 215.19 | 10 → 9–10 |
| Dense strip | 75 | 23.56 → 21.11 | 27.64 → 40.59 | 41.71 → 44.67 | 52 → 49 |
| Dense full deck | 75 | 67.25 → 60.29 | 80.14 → 72.13 | 14.55 → 16.31 | 132 → 123–124 |
| Dense key | 100 | 13.45 → 10.70 | 16.60 → 13.01 | 71.98 → 93.37 | 31 → 22 |
| Dense strip | 100 | 74.43 → 52.15 | 82.42 → 56.41 | 13.29 → 19.27 | 167–168 → 120 |
| Dense full deck | 100 | 192.58 → 127.14 | 214.62 → 152.98 | 5.24 → 7.75 | 415–416 → 296 |

A separate instrumented run measured 92–97% of HID write time inside the native
IOKit call. In the main run, the fork spent 0.03–0.18 ms at the median assembling
reports for a full-deck update.
