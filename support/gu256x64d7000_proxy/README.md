# GU256X64D-7000 Nano Every proxy

The Go driver sends a three-level framebuffer over 2 MHz SPI or USB CDC. The
Nano Every writes the VFD through its 8-bit parallel interface and generates PWM
by switching the VFD's visible image bank. Every pixel is off, on during one
phase, or on during both phases. A frame containing only black and white needs
one bank write and no periodic switching.

## Wiring

Set **JRB C–B** on the VFD so parallel pin 3 exposes PBUSY. Use the reset pin on
CN2. Tie the VFD's /RD input high for write-only operation and leave RS open.
Fit a pull-up to the VFD's 5 V on /WR and /RESET to keep them inactive while the
Nano boots. Use a regulated 5 V VFD supply rated for the module's startup surge,
with a shared ground. The VFD is specified for 4.75–5.25 V and draws up to
950 mA steady state at full output. Feed the VFD directly from its supply and
keep both sides powered while the parallel signals are driven.
[Noritake GU256X64D-7000 specification](https://noritake-itron.com/site2017/images/Specs/VFD/CDG/GU256x64D-7000x/gu256x64d-7000_e01-c5.pdf)

| VFD signal | VFD connector pin | Nano Every |
| --- | --- | --- |
| GND | Parallel 1 | GND |
| VCC | Parallel 2 | Regulated 5 V VFD supply |
| PBUSY | Parallel 3, JRB C–B | D6 |
| RS | Parallel 4 | Open |
| /WR | Parallel 5 | D4 |
| /RD | Parallel 6 | VFD 5 V |
| D0–D5 | Parallel 7–12 | A3, A2, A1, A0, A6, A7 respectively |
| D6–D7 | Parallel 13–14 | D9, D10 respectively |
| /RESET | CN2 pin 6 | D2 |

For SPI, ground Nano D7 before reset and wire host MOSI to Nano D11, MISO to D12,
SCK to D13, and active-low CS to **D8**. Use level translation in both directions
between a 3.3 V Raspberry Pi and the 5 V Nano Every. Leave D7 open at reset for
USB CDC and connect the Nano's USB port to the host.
[Arduino Nano Every pinout](https://docs.arduino.cc/resources/pinouts/ABX00028-full-pinout.pdf)

## Build and run

```sh
arduino-cli core install arduino:megaavr
arduino-cli compile --fqbn arduino:megaavr:nona4809:mode=off support/gu256x64d7000_proxy
arduino-cli upload --fqbn arduino:megaavr:nona4809:mode=off -p /dev/ttyACM0 support/gu256x64d7000_proxy
```

The Go constructors are `display.OpenGU256X64D7000ProxySPI("/dev/spidev0.0", nil)`
and `display.OpenGU256X64D7000ProxyUSB("/dev/ttyACM0", nil)`. The returned
`display.Display` owns its connection. It supports software rotation,
`Config.UseMono`, `Show`, and `SetContrast`.

```sh
go run ./cmd/display-test -contrast 224 proxy-spi gu256x64d7000
go run ./cmd/display-test -uart-port /dev/ttyACM0 proxy-usb gu256x64d7000
```

## Protocol

SPI is mode 0, MSB first, at 2 MHz. USB CDC uses the Arduino bridge at 115200
baud, 8N1. Each request starts with `ff`, followed by an opcode, a token
(1–125), the payload, and a three-byte CRC. The CRC is CRC-16/XMODEM over the
opcode, token, and payload, with initial value zero. Transmit CRC bits 0–6,
7–13, and 14–15 in that order.

| Opcode | Payload | VFD action |
| --- | --- | --- |
| 1 | 3,328 bytes | Publish a 256×64 frame |
| 2 | 0 or 1 | Display power, `US ( a 40h` |
| 3 | Level 1–8 | Global brightness, `US X` |

Frame data is column-major. Each column has 13 bytes. A byte encodes five
vertical pixels, with the top pixel as the least significant base-3 digit and
values 0, 1, 2 for off, half duty, and full duty. The last byte in each column
contains four pixels and a zero padding digit. This uses 1.625 wire bits per
pixel, including the per-column padding and excluding the packet header and CRC.

On SPI, send the start marker in its own transaction, then poll with four
zero bytes per transaction until the fourth received byte is `fe`. Send the
rest of the request in a single transaction and poll with four zero bytes per
transaction until the fourth received byte matches the token (success) or
token OR `80` (rejected). USB sends the request in one stream and receives
the same one-byte success or rejection reply. Send the next request after
the current reply. A new start marker can synchronize the next request after
an incomplete request times out.

## Timing and bench checks

At 2 MHz, the grayscale request consumes about 13.4 ms of SPI wire time.
A grayscale frame needs 4,096 VFD data writes plus commands, and timing depends
on the module's PBUSY latency. For near 20 fps, measure successive frame
acknowledgments with the SPI demo and check for about 20 completed updates per
second.
USB's 115200-baud bridge limits full frames to about 3.5 fps before VFD writes.

With a logic analyzer on /WR, PBUSY, SCK and CS, verify that the Nano waits for
PBUSY to clear and that both SPI and VFD transactions complete within the frame
budget. Display black, mid-gray, and white patches. Check that mid-gray pulses
while white stays on. For a stationary frame the nominal PWM period is 28 ms
before VFD command and scan latency. A SPI burst or bank upload can lengthen
a phase during animation. Actual panel flicker, duty cycle and frame rate
need a hardware measurement.
