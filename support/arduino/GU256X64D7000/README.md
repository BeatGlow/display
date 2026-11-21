# GU256X64D7000 Arduino library

Drive the Noritake GU256X64D-7000 over its 8-bit parallel interface from
an ATmega4809 board using an Arduino megaAVR core. The library includes
the VFD bus driver. The SPI and USB host protocol lives in the Nano Every
proxy sketch.

## Install

Copy this `GU256X64D7000` folder into the Arduino sketchbook's
`libraries` directory. To redistribute it, zip the folder with
`library.properties`, `src`, `examples`, and the license inside it.
Arduino IDE can install that ZIP using **Sketch > Include Library > Add
.ZIP Library**.

Open the `HelloVFD` example. It resets the panel, clears the display,
and writes text using the built-in font.

## Set the wiring

Use the AVR port letters and bit numbers from your board's pinout.
Choose 11 available GPIOs. Avoid pins reserved for the board's peripherals
or clock. The default constructor selects the Nano Every wiring.

| Signal | ATmega4809 port bits | Nano Every pins |
| --- | --- | --- |
| D0 through D5 | PD0 through PD5 | A3, A2, A1, A0, A6, A7 |
| D6 and D7 | PB0 and PB1 | D9 and D10 |
| PBUSY | PF4 | D6 |
| /WR | PC6 | D4 |
| /RESET | PA0 | D2 |

Use the VFD's PBUSY output and tie /RD high. Connect the Nano and VFD
grounds. Power the VFD from its own regulated 5 V supply and pull /WR
and /RESET up to the VFD supply so they stay inactive during MCU reset.

Pass a different wiring type to the constructor when the board uses
other port bits. The type lets the compiler resolve every register and
mask before the program runs. This example keeps the Nano Every data
bus and uses different control pins.

```cpp
#include <GU256X64D7000.h>

using namespace gu256x64d7000;
using BoardWiring = Wiring<
    SplitData<Port::D, 0, 6, Port::B, 0>,
    Pin<Port::F, 5>,  // PBUSY
    Pin<Port::B, 2>,  // /WR
    Pin<Port::A, 1>   // /RESET
>;
Display<BoardWiring> vfd{BoardWiring{}};
```

Choose the data bus type to match the wiring.

- `DataPort<Port::D>` writes a whole port at once. Connect VFD D0 through
  D7 to MCU bits 0 through 7 of a single dedicated port.
- `SplitData<Port::D, 0, 6, Port::B, 0>` uses two consecutive groups.
  The arguments are the low port, its first bit, the number of low
  data bits, the high port, and its first bit. Wire each group in order.
  Groups starting at bit 0 avoid extra shifts.
- `DataPins<Pin<...>, ...>` accepts eight individual pins in VFD data
  order. It still writes the AVR virtual ports directly. Scattered
  wiring takes more instructions to assemble the output bytes.

`writeBytes` captures the unrelated output bits once before sending.
Keep other code, including interrupt handlers, from changing output
latches on those ports during a burst. `write` captures them per byte.
Both wait for PBUSY before driving data and strobing /WR. The caller
handles a timeout by resetting and reinitializing the panel before retrying
a partially transmitted command.

`setCursor` and `beginBitmap` use native VFD coordinates. X counts
columns and Y counts vertical bands of eight pixels. After a successful
bitmap header, send exactly `columns * bands` data bytes, column first
and top band first, with the top pixel in bit 7.

The proxy benchmark uses the Nano Every wiring at 16 MHz. On a different
board, use a logic analyzer to verify PBUSY and /WR timing and measure
the rate with the chosen wiring.
