# GU256X64D-7000 Nano Every proxy

The Go driver sends monochrome, three-level, or four-level frames over 2 MHz
SPI or USB CDC. The Nano Every writes the VFD through its 8-bit parallel
interface. For grayscale it switches the visible VFD memory bank at each
phase that needs a different mask.

| Mode | Pixel codes | Nominal duty cycles | PWM phase lengths |
| --- | --- | --- | --- |
| Monochrome | `0`, `1` | 0%, 100% | Static |
| Three-level (1.5-bit gray) | `00`, `01`, `10` | 0%, 50%, 100% | 14 ms, 7 ms, 7 ms |
| Four-level (2-bit gray) | `00`, `01`, `10`, `11` | 0%, 33⅓%, 66⅔%, 100% | 14 ms each |

In three-level mode, gray lights during the first phase, and the last two
phases share the white-only mask. Four-level mode generates three distinct
masks from one packed framebuffer and loads each upcoming mask into the
hidden bank. White stays lit in every phase in either mode. A frame
containing only black and white needs one bank write and no PWM.

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

Use `display.OpenGU256X64D7000ProxySPI("/dev/spidev0.0", nil)` or
`display.OpenGU256X64D7000ProxyUSB("/dev/ttyACM0", nil)` for the default
three-level mode. Pass `&display.GU256X64D7000ProxyConfig{GrayLevels: 4}`
for four levels or
`&display.GU256X64D7000ProxyConfig{Config: display.Config{UseMono: true}}`
for 1-bit frames. The three-level framebuffer uses
`pixel.Gray3LevelImage`. The returned `display.Display` owns its connection
and supports software rotation, `Show`, and `SetContrast`.

```sh
go run ./cmd/display-test -contrast 224 proxy-spi gu256x64d7000
go run ./cmd/display-test -mono proxy-spi gu256x64d7000
go run ./cmd/display-test -proxy-gray-levels 4 proxy-spi gu256x64d7000
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
| 2 | 0 or 1 | Display power, `US ( a 40h` |
| 3 | Level 1–8 | Global brightness, `US X` |
| 4 | 4,096 bytes | Publish a three-level frame |
| 5 | 2,048 bytes | Publish a monochrome frame |
| 6 | 4,096 bytes | Publish a four-level frame |

Every frame is 256×64, stored column by column. A monochrome byte contains
eight vertical pixels, with the top pixel in bit 7. A grayscale byte contains
four vertical pixels, with the top pixel in bits 1–0. The firmware rejects
code `11` in a three-level frame. Wire storage uses two bits per grayscale
pixel in both modes.

On SPI, send the start marker in its own transaction, then poll with four
zero bytes per transaction until the fourth received byte is `fe`. Send the
rest of the request in transactions of at most 4,096 bytes, without sending
another start marker or polling between chunks. Then poll with four zero
bytes until the fourth received byte matches the token (success) or token OR
`80` (rejected). USB sends the request in one stream and receives the same
one-byte reply. Send the next request after the current reply. A new start
marker can synchronize the next request after an incomplete request times out.
If an invalid frame overwrites the Nano’s packed four-level buffer, the
current bank stays visible until the next valid frame restores PWM.

## Timing and bench checks

At 2 MHz, the monochrome request takes about 8.2 ms on the wire and either
grayscale request takes about 16.4 ms. Monochrome needs one 2,048-byte VFD
bitmap write. Three-level gray needs two bitmap writes per update. Four-level
gray needs a bitmap write for each distinct phase, including between host
updates. PBUSY latency on the panel determines the achievable rate. Near 20 fps
is the SPI target. At 115200 baud, the USB bridge limits transfer-only
throughput to about 5.6 monochrome fps or 2.8 grayscale fps.

Measure frame acknowledgments with the SPI demo and check for about 20 completed
updates per second. With a logic analyzer on /WR, PBUSY, SCK, and CS, confirm
that the Nano waits for PBUSY and that each transaction completes within the
frame budget. Verify the duty cycles on solid patches in each grayscale mode.
During animation, SPI bursts and bank uploads can lengthen a phase. The visible
PWM, flicker, and achieved frame rate require a bench measurement.
