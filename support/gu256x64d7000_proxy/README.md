# GU256X64D-7000 Nano Every proxy

The Go driver sends monochrome, three-level, or four-level frames over 2 MHz
SPI or USB CDC. The Nano Every drives the VFD over its 8-bit parallel
interface and switches the visible memory bank when the PWM mask changes.

| Go framebuffer | Pixel levels | Nominal duty cycles | PWM phase lengths |
| --- | --- | --- | --- |
| `pixel.MonoVerticalLSBImage` | 0, 1 | 0%, 100% | Static |
| `pixel.Gray3LevelImage` | 0, 1, 2 | 0%, 50%, 100% | 14 ms, 7 ms, 7 ms |
| `pixel.Gray2Image` | 0, 1, 2, 3 | 0%, 33⅓%, 66⅔%, 100% | 14 ms each |

Three-level mode lights gray pixels during the first phase. The last two
phases share the white-only mask. Four-level mode generates three masks
from one packed framebuffer and loads the next mask into the hidden bank.
White stays lit in every phase. A black-and-white frame needs a single
bank write and runs without PWM.

## Wiring

Set **JRB C–B** on the VFD so parallel pin 3 exposes PBUSY. Use the reset pin on
CN2. Tie the VFD's /RD input high for write-only operation and leave RS open.
Pull /WR and /RESET up to the VFD's 5 V so they stay inactive while the
Nano boots. Power the VFD directly from a regulated 5 V supply that can
handle its startup surge. Its operating range is 4.75 to 5.25 V, and it
draws up to 950 mA at full output in steady state. Share ground with the
Nano and keep both devices powered while the parallel signals are active.
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

After cloning the display repository, fetch the pinned library submodule.

```sh
git submodule update --init -- support/gu256x64d7000_proxy/data/GU256X64D7000
```

In Arduino IDE, install **Arduino megaAVR Boards** and open
`support/gu256x64d7000_proxy/gu256x64d7000_proxy.ino`. Select **Arduino Nano
Every** and set **Registers emulation** to **None (ATMEGA4809)**.
Use **Verify** to build or select a port and use **Upload**. The sketch
includes the pinned library from its `data` directory, which keeps the
library's host tests out of the Arduino build.

```sh
arduino-cli core install arduino:megaavr
arduino-cli compile --fqbn arduino:megaavr:nona4809:mode=off support/gu256x64d7000_proxy
arduino-cli upload --fqbn arduino:megaavr:nona4809:mode=off \
  -p /dev/ttyACM0 support/gu256x64d7000_proxy
```

The [GU256X64D7000 Arduino library](https://github.com/BeatGlow/GU256X64D7000)
also supports standalone ATmega4809 sketches with custom wiring.

Use `display.OpenGU256X64D7000ProxySPI("/dev/spidev0.0", nil)` or
`display.OpenGU256X64D7000ProxyUSB("/dev/ttyACM0", nil)` for the default
three-level mode. Pass `&display.GU256X64D7000ProxyConfig{GrayLevels: 4}`
for four levels or
`&display.GU256X64D7000ProxyConfig{Config: display.Config{UseMono: true}}`
for 1-bit frames. The returned `display.Display` owns the connection.
It supports software rotation, `Show`, and `SetContrast`.

```sh
go run ./cmd/display-test -contrast 224 proxy-spi gu256x64d7000
go run ./cmd/display-test -mono proxy-spi gu256x64d7000
go run ./cmd/display-test -proxy-gray-levels 4 proxy-spi gu256x64d7000
go run ./cmd/display-test -uart-port /dev/ttyACM0 proxy-usb gu256x64d7000
```

## Protocol

### Request format

Both transports use the same request bytes. Send the start marker first
and wait for the Nano to report ready before sending the body.

```text
Octet offset
    0       1       2          3          N+3     N+4     N+5
+-------+-------+-------+--------------+-------+-------+-------+
|   FF  | Opcode| Token |   Payload    |  CRC0 |  CRC1 |  CRC2 |
+-------+-------+-------+--------------+-------+-------+-------+
        |<------------------- request body ------------------->|
                        |<- N octets ->|
        |<-------- CRC input --------->|
```

Use a token from 1 through 125. Fixed-size opcodes determine the payload
length `N`. The drawing opcodes carry their own length as the first two
payload octets. Compute CRC-16/XMODEM with initial value zero over the
opcode, token, and entire payload, including the drawing length when
present. The Nano keeps the CRC lookup table in flash.

```text
              7 6 5 4 3 2 1 0
             +-+-+-+-+-+-+-+-+
 CRC0        |-|6|5|4|3|2|1|0|
             +-+-+-+-+-+-+-+-+
 CRC1        |-|D|C|B|A|9|8|7|
             +-+-+-+-+-+-+-+-+
 CRC2        |-|-|-|-|-|-|F|E|
             +-+-+-+-+-+-+-+-+
 A hex digit labels a bit of the 16-bit CRC.
 A dash marks a bit that is set to zero.
```

| Opcode | Payload | VFD action |
| --- | --- | --- |
| `0x02` | 2,048 bytes | Publish a monochrome frame |
| `0x03` | 3,328 bytes | Publish a three-level Gray3Level frame |
| `0x04` | 4,096 bytes | Publish a four-level Gray2 frame |
| `0x0F` | Empty | Read the Nano proxy protocol version |
| `0x21` | 2-byte length + bitmap body | Draw a monochrome rectangle with `US ( f 11h` |
| `0x34` | 2-byte length + text body | Position the cursor with `US $` (`0x24`) and draw text |
| `0x50` | 0 or 1 | Display power, `US ( a 40h` |
| `0x68` | Level 1–8 | Global brightness, `US X` (`0x58`) |

Control opcodes start at `0x10`. Add `0x10` to the VFD command byte. For
a grouped command such as display power, add it to the subcommand byte.

Update the firmware and Go driver together when upgrading from an earlier
proxy opcode map.

### Framebuffer payloads

Send the 256 columns from left to right. Within each column, send the
byte groups from top to bottom across all 64 rows.

```text
                 increasing payload offset -->
+-------------+-------------+-----+-------------+
|  column 0   |  column 1   | ... | column 255  |
+-------------+-------------+-----+-------------+
```

`p0` is the top pixel in each group. The other pixels follow down the
same column. Bit 7 is on the left.

#### Mono (`0x02`)

Pack eight pixels per octet and eight octets per column. The first octet
covers rows 0 through 7. A group with only its top pixel lit encodes as `0x80`.

```text
  7   6   5   4   3   2   1   0
+---+---+---+---+---+---+---+---+
|p0 |p1 |p2 |p3 |p4 |p5 |p6 |p7 |
+---+---+---+---+---+---+---+---+
```

#### Gray3Level (`0x03`)

Pack five base-3 digits per octet and 13 octets per column. A pixel value
of 0 is off, 1 is half brightness, and 2 is full brightness. Store the
weighted sum as an unsigned octet.

```text
  base-3 weights, most significant digit on the left
+------+------+------+------+------+
| 81*p4| 27*p3|  9*p2|  3*p1|    p0|
+------+------+------+------+------+
      add the five terms to form B

  7   6   5   4   3   2   1   0
+---+---+---+---+---+---+---+---+
| 1 | 1 | 0 | 1 | 0 | 0 | 1 | 0 |  B = 0xD2
+---+---+---+---+---+---+---+---+
  example pixels (p0..p4) = (0, 1, 2, 1, 2)
```

The first 12 octets cover rows 0 through 59 and can hold values up to 242.
The last covers rows 60 through 63. Set `p4 = 0` to keep its value at or
below 80. The firmware checks both limits and expands each group with
fixed shifts and a shared lookup table in RAM.

#### Gray2 (`0x04`)

Pack four two-bit pixels per octet and 16 octets per column. The first
octet covers rows 0 through 3. Each pixel has a value from 0 to 3. Pixels
`(p0, p1, p2, p3) = (0, 1, 2, 3)` encode as `0xE4`.

```text
  7   6   5   4   3   2   1   0
+-------+-------+-------+-------+
|  p3   |  p2   |  p1   |  p0   |
+-------+-------+-------+-------+
```

### Native drawing payloads

Native drawing updates the visible VFD bank. The proxy rejects these
requests while grayscale PWM is active. To leave PWM, send a frame
with only black and white pixels. Native drawing uses physical coordinates
regardless of software rotation. It leaves the Go framebuffer as it was,
so the next `Refresh` replaces the drawing.

Use `d.(display.GU256X64D7000ProxyDisplay)` to access
`DrawBitmap(at, bitmap)` and `DrawTextAt(at, text)` on the display returned by either opener.
The Go methods take pixel coordinates. Align y and bitmap height to
multiples of eight. On the wire, one band is eight vertical pixels.

#### Rectangular monochrome bitmap (`0x21`)

```text
    0        1        2        3        4        5        6       7
+--------+--------+--------+--------+--------+--------+--------+------+
| Len lo | Len hi |   X    | Y band |Width lo|Width hi| Height | Data |
+--------+--------+--------+--------+--------+--------+--------+------+
                  |<------------------- Len octets ------------------>|
```

Width counts columns and height counts bands. Both must be positive,
and the rectangle must fit within 256 columns and 8 bands. Send the data
in the same order as Mono frame data, with bit 7 at the top of each band.
`Len = 5 + width * height`, with a maximum of 2,053. The rectangle
replaces its destination pixels, including pixels whose bits are zero.
The Go method converts source colors with `pixel.MonoModel`.

#### Positioned text (`0x34`)

```text
    0        1        2        3              4
+--------+--------+--------+--------+-------------------+
| Len lo | Len hi |   X    | Y band |  ASCII characters |
+--------+--------+--------+--------+-------------------+
                  |<------------ Len octets ----------->|
```

`Len = 2 + character count`. Send at least one printable ASCII character
(`0x20` through `0x7E`). The built-in font uses seven columns per character
including spacing, so the text must fit on one line from X to the right
edge. The wire payload carries the text octets without a terminator.

### Request exchange

The Nano returns the token on success or sets bit 7 (`token | 0x80`) on
rejection. Wait for that reply before starting another request. If a
partial request times out, send a fresh start marker to resynchronize.

If an invalid frame overwrites the packed four-level buffer, the Nano
keeps the current bank visible until another valid frame restores PWM.

#### SPI

Use mode 0 at 2 MHz and send the most significant bit first. Assert CS
for each MOSI transfer. During a poll, MOSI and MISO travel at the same time.

```text
 Host (controller)                                  Nano (peripheral)
       |                                                     |
       |--- MOSI: FF --------------------------------------->|
       |                                                     |
       |--- MOSI: 00 00 00 00 ------------------------------>|
       |<-- MISO: xx xx xx FE -------------------------------|  ready
       |                                                     |
       |--- MOSI: opcode, token, payload, CRC -------------->|
       |           (one or more transactions)                |
       |                                                     |
       |--- MOSI: 00 00 00 00 ------------------------------>|
       |<-- MISO: xx xx xx status ---------------------------|  result
       |                                                     |
       v                                                     v
```

Repeat the four-octet poll until the fourth MISO octet reports ready,
success, or rejection as appropriate. Ignore the first three MISO octets
(`xx`), and ignore MISO during the start and body transfers. Send the body
in transactions of up to 4,096 octets, with no polls between them.

#### USB CDC

Use the stock Nano Every bridge at 1,000,000 baud, 8N1. Status bytes
arrive on the serial receive stream.

```text
 Host                                               Nano (USB CDC)
   |                                                       |
   |--- FF ----------------------------------------------->|
   |<-- FE ------------------------------------------------|  ready
   |--- opcode, token, payload, CRC ---------------------->|
   |<-- status --------------------------------------------|  result
   |                                                       |
   v                                                       v
```

Opening the serial port can reset the Nano. Retry the start marker after
a read timeout while waiting for `FE`. Once ready, send the body once and
wait for either result for the current token.

### Protocol version query (`0x0F`)

Send a request with an empty payload. Its CRC covers only the opcode
and token. A successful query returns the normal acknowledgment
followed by a big-endian unsigned 16-bit version. The first byte is
the major version and the second is the minor version. The current
version is 1.0 (`0x0100`). Change the major version when changing existing
protocol rules or opcode meanings. Increment the minor version for
compatible features within the same major version.

Use `ProtocolVersion()` on `display.GU256X64D7000ProxyDisplay` to query
either transport. It identifies the Nano proxy protocol. The VFD itself
documents only a busy flag on parallel status reads.

```text
 USB CDC reply stream

+---------+------------+------------+
|  token  |   major    |   minor    |
+---------+------------+------------+
    0           1            2
```

Read the USB version bytes immediately after the acknowledgment. If
the Nano rejects the request, it sends only the rejection status.

For SPI, poll for the acknowledgment as usual. Then use these separate transactions to read the version and release the
reply.

```text
 MOSI                      MISO
+-------------+           +-----------------+
| 01 01 01 01 |---------->| xx xx xx major  |
+-------------+           +-----------------+
+-------------+           +-----------------+
| 02 02 02 02 |---------->| xx xx xx minor  |
+-------------+           +-----------------+
+-------------+
| 03          |---------->  release reply
+-------------+
+-------------+           +-----------------+
| 00 00 00 00 |---------->| xx xx xx FE     |  release confirmed
+-------------+           +-----------------+
```

Ignore the first three MISO octets of each read. Repeat the final
zero poll until the Nano confirms release with `FE`. If release times
out, discard the version bytes. Skip these transactions after a request
rejection. A new start marker can recover an abandoned read. The Nano
resumes PWM service on release or timeout.

## Timing and bench checks

Estimated rates count acknowledgments for 256×64 updates. They add
payload transfer time to the CRC and frame publication time measured in
MPLAB X at 16 MHz.

| Pixel format | Payload | SPI at 2 MHz | USB CDC at 1 Mbaud, 8N1 |
| --- | ---: | ---: | ---: |
| Mono | 2,048 bytes | 62.6 fps (15.96 ms) | 35.4 fps (28.25 ms) |
| Gray3Level (3 levels) | 3,328 bytes | 24.6 fps (40.72 ms) | 16.5 fps (60.68 ms) |
| Gray2 (4 levels) | 4,096 bytes | 33.1 fps (30.17 ms) | 18.3 fps (54.75 ms) |

The benchmark measures an initialized proxy with PBUSY held ready and
grayscale lookup tables already built. Panel busy time, the protocol
handshake, host scheduling, and USB bridge buffering add latency on hardware.
Gray3Level writes two bitmaps for a frame containing gray pixels. Gray2
publishes the first mask, then prepares later PWM masks between updates.
Preparing the next Gray2 mask takes about 8 ms of simulated CPU time
after acknowledgment and can lower the sustained rate. The SPI target is
about 20 fps. Arduino documents 1 Mbit/s for the
stock Nano Every bridge.
[Arduino Nano Every serial ports](https://docs.arduino.cc/tutorials/nano-every/getting-started/#serial-ports-on-the-arduino-nano-every)

Count acknowledgments with the SPI demo to measure completed updates.
Probe /WR, PBUSY, SCK, and CS with a logic analyzer to check that the Nano
waits for PBUSY and that transfers fit the frame budget. Check the PWM
duty cycles on solid patches in each grayscale mode and watch for flicker
during animation. SPI transfers and bank writes can lengthen a phase.

## MPLAB X benchmark

The [benchmark](benchmark/run.py) builds the ATmega4809 sketch and times
it with the MDB simulator at 16 MHz. Supply the paths to MPLAB X `mdb.sh`
and Arduino megaAVR `avr-g++`. The scratch directory holds the fixtures,
ELF, MDB command file, and simulator log.

```sh
python3 support/gu256x64d7000_proxy/benchmark/run.py \
  --mdb /path/to/mplab_platform/bin/mdb.sh \
  --avr-cxx /path/to/arduino/tools/avr-gcc/bin/avr-g++ \
  --work-dir /path/to/scratch/gu256x64d7000-benchmark
```

The benchmark checks CRC against the AVR library and tests rejection of
invalid Gray3Level digits and column padding. It times the work through
acknowledgment and reports the next Gray2 PWM mask separately. The totals
add payload wire time to the simulated CPU time with PBUSY held ready.
Measure handshake, bridge latency, and VFD busy timing on the hardware.
