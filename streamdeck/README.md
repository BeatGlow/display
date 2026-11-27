# Stream Deck

The package controls the following Elgato Stream Deck models on Linux and
macOS. It uses one USB HID connection for input and output and builds with
`CGO_ENABLED=0` on both platforms.

| Model | USB product ID | Keys | Key image | Protocol reference |
| --- | --- | --- | --- | --- |
| Stream Deck Original (2017) | `0x0060` | 5 × 3 | 72 × 72 BMP | [Original driver reference](https://github.com/abcminiuser/python-elgato-streamdeck/blob/master/src/StreamDeck/Devices/StreamDeckOriginal.py) |
| Stream Deck Mini | `0x0063` | 3 × 2 | 80 × 80 BMP | [Elgato Mini](https://docs.elgato.com/streamdeck/hid/mini/) |
| Stream Deck XL | `0x006c` | 8 × 4 | 96 × 96 JPEG | [Elgato XL](https://docs.elgato.com/streamdeck/hid/stream-deck-xl/) |
| Stream Deck 2019 | `0x006d` | 5 × 3 | 72 × 72 JPEG | [Elgato Classic](https://docs.elgato.com/streamdeck/hid/stream-deck-classic/) |
| Stream Deck MK.2 | `0x0080` | 5 × 3 | 72 × 72 JPEG | [Elgato Classic](https://docs.elgato.com/streamdeck/hid/stream-deck-classic/) |
| Stream Deck + | `0x0084` | 4 × 2 | 120 × 120 JPEG | [Elgato Plus](https://docs.elgato.com/streamdeck/hid/stream-deck-plus/) |
| Stream Deck Mini 2022 | `0x0090` | 3 × 2 | 80 × 80 BMP | [Elgato Mini](https://docs.elgato.com/streamdeck/hid/mini/) |

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
