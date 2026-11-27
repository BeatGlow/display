# Stream Deck

The package exposes the Stream Deck + as eight 120 × 120 image keys, four
push-button dials, and an 800 × 100 touch strip. It uses one USB HID connection
for input and output on Linux and macOS. Both backends build with
`CGO_ENABLED=0`.

Call `streamdeck.Devices()` to list supported devices and their model
capabilities. Pass a listed path to `streamdeck.Open`, or use `Open("")` when only
one supported device is attached. On Linux, the process needs read and write
access to the device's `/dev/hidraw*` node. On macOS, the process needs permission
to open the device through IOKit.

Draw key and strip images into standard Go images sized using
`deck.Info().Model.KeySize` and `deck.Info().Model.StripSize`. Send them with
`SetKeyImage` and `SetStripImage`. The driver encodes and transfers the images.
Keys are numbered from zero, left to right and then top to bottom. Dials are
numbered from zero, left to right.

Use `ReadEvent(ctx)` for key and dial presses and releases, signed dial
rotation, and touchscreen gestures. Touch coordinates are in strip pixels.
`ReadEvent` can run while images are being sent and can resume after its context
is canceled. `Close` interrupts pending reads and releases the HID connection.
