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
| Stream Deck + | `0x0084` | 4 × 2 | 120 × 120 JPEG | [Elgato Plus](https://docs.elgato.com/streamdeck/hid/stream-deck-plus/) | 22.18 / 20.66 | 15.10 / 5.46 |
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

On a HID write failure, the driver can retry the whole image twice on the same
handle, after 10 ms and 20 ms. Closed-device and permission errors return
immediately. The application owns reconnecting a disconnected device.

The Stream Deck + also exposes four push-button dials and an 800 × 100 touch
strip. Dials are numbered from zero, left to right. Send an image sized to
`deck.Info().Model.StripSize` with `SetStripImage`.

Use `ReadEvent(ctx)` for key and dial presses and releases, signed dial rotation,
and touchscreen gestures. Touch coordinates are in strip pixels. `ReadEvent`
can run while images are being sent and can resume after its context is
canceled. `Close` interrupts pending reads and releases the HID connection.

## Upload FPS

The Stream Deck + was tested on Apple Silicon macOS with Go 1.27.3 and
`CGO_ENABLED=0`. The benchmark used pre-rendered RGBA images with typical
control graphics and dense RGB noise. JPEG quality 75 is the driver default.
Quality 100 was measured in a temporary benchmark build.

Upload FPS is completed updates divided by the sum of their measured durations.
A full-deck update sends eight keys and the 800 × 100 strip sequentially.
Each key and strip workload had 60 samples per quality. Each full-deck workload
had 32 samples per quality. The paired tests ran in both quality orders.

| Workload | Q75 upload FPS | Q100 upload FPS | Q75 median (ms) | Q100 median (ms) | Q75 p95 (ms) | Q100 p95 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Typical key | 244.25 | 282.61 | 1.38 | 2.74 | 2.65 | 4.50 |
| Typical strip | 56.86 | 53.88 | 6.27 | 13.12 | 124.38 | 52.48 |
| Typical full deck | 22.18 | 20.66 | 22.85 | 41.28 | 108.53 | 78.08 |
| Dense key | 200.68 | 69.31 | 4.99 | 13.89 | 6.12 | 17.25 |
| Dense strip | 40.95 | 13.69 | 24.24 | 71.62 | 27.23 | 81.65 |
| Dense full deck | 15.10 | 5.46 | 65.61 | 179.86 | 78.44 | 211.99 |

Timing covers JPEG encoding and HID writes. LCD refresh remains unmeasured.
Intermittent stalls in the quality-75 run affected throughput and p95 for
typical graphics.
