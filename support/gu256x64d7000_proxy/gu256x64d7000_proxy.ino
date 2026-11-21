#include <Arduino.h>
#include "xmodem_crc.h"

#if !defined(ARDUINO_AVR_NANO_EVERY)
#error "Select Arduino Nano Every (arduino:megaavr:nona4809)"
#endif

namespace {
constexpr uint8_t startByte = 0xff;
constexpr uint8_t readyByte = 0xfe;
constexpr uint32_t usbBaud = 1000000;
constexpr uint8_t showOp = 2;
constexpr uint8_t brightnessOp = 3;
constexpr uint8_t gray3Op = 7;
constexpr uint8_t monoOp = 5;
constexpr uint8_t gray2Op = 6;
constexpr uint16_t monoSize = 256 * 64 / 8;
constexpr uint16_t width = 256;
constexpr uint8_t gray3Column = 13;
constexpr uint16_t gray3Size = width * gray3Column;
constexpr uint16_t gray2Size = width * 64 / 4;
constexpr uint8_t gray3PhaseMs[] = {14, 7, 7};
constexpr uint8_t gray2PhaseMs = 14;

uint8_t frame[gray2Size];
// Both grayscale formats use the same RAM for their mask lookup tables.
uint8_t litPixels[256];
uint8_t fullPixels[256];
uint8_t maskFormat;
enum class Bitmap : uint8_t { Mono, Gray3Lit, Gray3Full, Lit, HighBit, Full };
constexpr Bitmap gray2Bitmaps[] = {Bitmap::Lit, Bitmap::HighBit, Bitmap::Full};

bool useSPI;
bool vfdReady;
bool visible = true;
uint8_t brightness = 8;
uint8_t visibleBank;
uint8_t pwmMode = monoOp;
uint8_t pwmPhase;
uint32_t phaseStarted;
bool gray2FrameValid;
bool gray2NextReady;

bool waitBusyVFD() {
  // Poll briefly before using millis() for the timeout.
  uint8_t spin = 0;
  while (VPORTF.IN & PIN4_bm) {
    if (++spin == 0) {
      const uint32_t started = millis();
      while (VPORTF.IN & PIN4_bm) {
        if (millis() - started >= 120) {
          vfdReady = false;
          return false;
        }
      }
      break;
    }
  }
  return true;
}

inline bool waitVFD() {
  return !(VPORTF.IN & PIN4_bm) || waitBusyVFD();
}

inline __attribute__((always_inline)) void strobeVFD(uint8_t value, uint8_t portD, uint8_t portB) {
  // VFD D0..5 = PD0..5 and VFD D6..7 = PB0..1.
  VPORTD.OUT = portD | (value & 0x3f);
  VPORTB.OUT = portB | (value >> 6);
  VPORTC.OUT &= ~PIN6_bm;  // /WR (D4)
  asm volatile("nop\n\tnop" ::: "memory");
  VPORTC.OUT |= PIN6_bm;
  asm volatile("nop\n\tnop" ::: "memory");
}

inline __attribute__((always_inline)) bool writeVFD(uint8_t value) {
  if (!waitVFD()) {
    return false;
  }
  strobeVFD(value, VPORTD.OUT & 0xc0, VPORTB.OUT & 0xfc);
  return true;
}

inline __attribute__((always_inline)) bool writeBitmapByte(uint8_t value, uint8_t portD, uint8_t portB) {
  if (!waitVFD()) {
    return false;
  }
  strobeVFD(value, portD, portB);
  return true;
}

template <size_t N>
bool command(const uint8_t (&bytes)[N]) {
  for (uint8_t value : bytes) {
    if (!writeVFD(value)) {
      return false;
    }
  }
  return waitVFD();
}

bool setPower(bool enabled) {
  return command<5>({0x1f, 0x28, 0x61, 0x40, uint8_t(enabled)});
}

bool setBrightness(uint8_t level) {
  return command<3>({0x1f, 0x58, level});
}

bool initVFD() {
  visibleBank = 0;
  pwmMode = monoOp;
  pwmPhase = 0;
  gray2FrameValid = false;
  gray2NextReady = false;
  VPORTA.OUT &= ~PIN0_bm;  // /RESET (D2)
  delay(2);
  VPORTA.OUT |= PIN0_bm;
  delay(1);
  vfdReady = command<3>({0x1b, 0x40, 0x0c}) &&
             command<5>({0x1f, 0x28, 0x77, 0x10, 1}) &&  // Write across both banks.
             setBrightness(brightness) && setPower(visible);
  return vfdReady;
}

bool switchBank() {
  // Shift by 2048 display-memory bytes (256 columns) once, at speed zero.
  if (!command<9>({0x1f, 0x28, 0x61, 0x10, 0, 8, 1, 0, 0})) {
    return false;
  }
  visibleBank ^= 1;
  return true;
}

void prepareMasks(uint8_t format) {
  if (maskFormat == format) {
    return;
  }
  for (uint16_t packed = 0; packed < 256; ++packed) {
    uint8_t value = packed;
    uint8_t lit = 0;
    uint8_t full = 0;
    const bool ternary = format == gray3Op;
    for (uint8_t bit = ternary ? 16 : 8; bit; bit >>= 1) {
      const uint8_t level = ternary ? value % 3 : value & 3;
      value = ternary ? value / 3 : value >> 2;
      if (level != 0) {
        lit |= bit;
      }
      if (level == (ternary ? 2 : 3)) {
        full |= bit;
      }
    }
    litPixels[packed] = lit;
    fullPixels[packed] = full;
  }
  maskFormat = format;
}

template <bool FullGroup>
inline __attribute__((always_inline)) bool writeGray3Group(
    const uint8_t *src, const uint8_t *mask, uint8_t portD, uint8_t portB) {
  const uint8_t a = mask[src[0]], b = mask[src[1]], c = mask[src[2]];
  const uint8_t d = mask[src[3]], e = mask[src[4]];
  // Five base-3 digits decode to five vertical mask bits. The first
  // five groups contribute 24 bits, with one bit carried into byte four.
  if (!writeBitmapByte((a << 3) | (b >> 2), portD, portB) ||
      !writeBitmapByte((b << 6) | (c << 1) | (d >> 4), portD, portB) ||
      !writeBitmapByte((d << 4) | (e >> 1), portD, portB)) {
    return false;
  }
  if (!FullGroup) {
    return true;  // The 25th bit is padding in the last group of the column.
  }
  const uint8_t f = mask[src[5]], g = mask[src[6]], h = mask[src[7]];
  return writeBitmapByte((e << 7) | (f << 2) | (g >> 3), portD, portB) &&
         writeBitmapByte((g << 5) | h, portD, portB);
}

bool writeBank(uint8_t bank, Bitmap bitmap) {
  if (!command<15>({0x1f, 0x24, 0, bank, 0, 0,  // Cursor (bank*256, 0).
                0x1f, 0x28, 0x66, 0x11, 0, 1, 8, 0, 1})) {
    return false;
  }
  // No other code writes these latches during the bitmap transfer.
  const uint8_t portD = VPORTD.OUT & 0xc0;
  const uint8_t portB = VPORTB.OUT & 0xfc;
  if (bitmap == Bitmap::Mono) {
    for (uint16_t i = 0; i < monoSize; ++i) {
      if (!writeBitmapByte(frame[i], portD, portB)) {
        return false;
      }
    }
  } else if (bitmap == Bitmap::Gray3Lit || bitmap == Bitmap::Gray3Full) {
    const uint8_t *mask = bitmap == Bitmap::Gray3Full ? fullPixels : litPixels;
    for (const uint8_t *column = frame; column != frame + gray3Size; column += gray3Column) {
      if (!writeGray3Group<true>(column, mask, portD, portB) ||
          !writeGray3Group<false>(column + 8, mask, portD, portB)) {
        return false;
      }
    }
  } else {
    const uint8_t *mask = bitmap == Bitmap::Full ? fullPixels : litPixels;
    const uint8_t bits = bitmap == Bitmap::HighBit ? 0xaa : 0xff;
    for (uint16_t i = 0; i < gray2Size; i += 2) {
      if (!writeBitmapByte((mask[frame[i] & bits] << 4) | mask[frame[i + 1] & bits], portD, portB)) {
        return false;
      }
    }
  }
  return waitVFD();
}

uint8_t nextPhase() {
  return pwmPhase == 2 ? 0 : pwmPhase + 1;
}

void servicePWM() {
  if (!vfdReady || !visible || pwmMode == monoOp) {
    return;
  }

  const uint8_t next = nextPhase();
  if (pwmMode == gray2Op) {
    if (!gray2FrameValid) {
      return;
    }
    if (!gray2NextReady) {
      // Reuse the hidden bank for the next of three distinct masks.
      gray2NextReady = writeBank(visibleBank ^ 1, gray2Bitmaps[next]);
      if (!gray2NextReady) {
        return;
      }
    }
  }

  const uint8_t duration = pwmMode == gray3Op ? gray3PhaseMs[pwmPhase] : gray2PhaseMs;
  if (millis() - phaseStarted < duration) {
    return;
  }
  // Three-level phases 1 and 2 share the full-brightness mask.
  if (pwmMode == gray3Op && pwmPhase == 1) {
    pwmPhase = next;
    phaseStarted += duration;
    return;
  }
  if (switchBank()) {
    pwmPhase = next;
    phaseStarted = millis();
    gray2NextReady = false;
  }
}

bool showFrame(uint8_t op) {
  bool hasGray = false;
  if (op == gray3Op) {
    prepareMasks(op);
    for (const uint8_t *column = frame; column != frame + gray3Size; column += gray3Column) {
      if (column[gray3Column - 1] >= 81) {
        return false;  // The fifth digit of the last byte must be zero.
      }
      for (uint8_t group = 0; group < gray3Column; ++group) {
        const uint8_t packed = column[group];
        if (packed >= 243) {
          return false;
        }
        if (!hasGray) {
          hasGray = litPixels[packed] != fullPixels[packed];
        }
      }
    }
  } else if (op == gray2Op) {
    prepareMasks(op);
    for (uint16_t i = 0; i < gray2Size; ++i) {
      const uint8_t packed = frame[i];
      if ((packed ^ (packed >> 1)) & 0x55) {
        hasGray = true;
        break;
      }
    }
  }
  if (!vfdReady && !initVFD()) {
    return false;
  }

  const uint8_t mode = hasGray ? op : monoOp;
  uint8_t phase = 0;
  if (mode == pwmMode && hasGray) {
    phase = pwmPhase;
    if (mode == gray2Op && millis() - phaseStarted >= gray2PhaseMs) {
      phase = nextPhase();
    }
  }

  Bitmap bitmap = Bitmap::Mono;
  if (op == gray3Op) {
    bitmap = phase == 0 ? Bitmap::Gray3Lit : Bitmap::Gray3Full;
  } else if (op == gray2Op) {
    bitmap = gray2Bitmaps[phase];
  }
  if (!writeBank(visibleBank ^ 1, bitmap) || !switchBank()) {
    return false;
  }
  if (mode != pwmMode || phase != pwmPhase) {
    phaseStarted = millis();
  }
  if (mode == gray3Op && !writeBank(visibleBank ^ 1, phase == 0 ? Bitmap::Gray3Full : Bitmap::Gray3Lit)) {
    return false;
  }
  pwmMode = mode;
  pwmPhase = phase;
  gray2FrameValid = mode == gray2Op;
  gray2NextReady = false;
  return true;
}

bool execute(uint8_t op, uint8_t value) {
  if (op == monoOp || op == gray3Op || op == gray2Op) {
    return showFrame(op);
  }
  if ((op == showOp && value > 1) || (op == brightnessOp && (value < 1 || value > 8))) {
    return false;
  }
  if (!vfdReady && !initVFD()) {
    return false;
  }
  if (op == showOp && setPower(value)) {
    visible = value;
    return true;
  }
  if (op == brightnessOp && setBrightness(value)) {
    brightness = value;
    return true;
  }
  return false;
}

// A SPI status byte is queued for the next clocked byte. The host supplies
// four dummy bytes when polling to drain any older transmit-buffer contents.
int pollSPI(uint8_t status) {
  if (SPI0.INTFLAGS & SPI_DREIF_bm) {
    SPI0.DATA = status;
  }
  if (SPI0.INTFLAGS & SPI_RXCIF_bm) {
    return SPI0.DATA;
  }
  return -1;
}

// During the 2 MHz burst foreground code owns SPI. The receive buffer has two
// entries. Check the deadline only when waiting for a byte.
int receiveByte(uint32_t started) {
  if (useSPI) {
    uint8_t spin = 0;
    while (!(SPI0.INTFLAGS & SPI_RXCIF_bm)) {
      if (++spin == 0 && millis() - started >= 150) {
        return -1;
      }
    }
    return SPI0.DATA;
  }
  // The USB host waits for readiness, then sends a continuous burst.
  // Drain the UART before running another potentially blocking VFD write.
  while (true) {
    const int value = Serial.read();
    if (value >= 0) {
      return value;
    }
    if (millis() - started >= 250) {
      return -1;
    }
  }
}

uint8_t receiveRequest() {
  uint32_t started = millis();
  int op;
  if (useSPI) {
    uint8_t spin = 0;
    do {
      op = pollSPI(readyByte);
      if (++spin == 0 && millis() - started >= 150) {
        return 0;
      }
    } while (op <= 0 || op == startByte);  // Ignore readiness polling and a new start.
  } else {
    do {
      started = millis();
      Serial.write(readyByte);
      op = receiveByte(started);
    } while (op == startByte);  // A retried marker may arrive while waiting for the opcode.
  }
  const int token = receiveByte(started);
  if (token < 1 || token > 125) {
    return 0;
  }
  const uint8_t reject = token | 0x80;
  uint8_t control = 0;
  uint8_t *payload = &control;
  uint16_t size = 1;
  switch (op) {
    case gray3Op:
      size = gray3Size;
      break;
    case gray2Op:
      size = gray2Size;
      break;
    case monoOp:
      size = monoSize;
      break;
    case showOp:
    case brightnessOp:
      break;
    default:
      return reject;
  }
  if (size != 1) {
    // Keep displaying the current bank if an interrupted transfer overwrites
    // the packed data needed to prepare further four-level phases.
    gray2FrameValid = false;
    payload = frame;
  }
  for (uint16_t i = 0; i < size; ++i) {
    int value = receiveByte(started);
    if (value < 0) {
      return reject;
    }
    payload[i] = value;
  }
  uint16_t received = 0;
  for (uint8_t shift = 0; shift < 16; shift += 7) {
    int value = receiveByte(started);
    if (value < 0 || value > (shift == 14 ? 3 : 127)) {
      return reject;
    }
    received |= uint16_t(value) << shift;
  }
  if (packetCRC(op, token, payload, size) != received) {
    return reject;
  }
  servicePWM();
  return execute(op, control) ? token : reject;
}
}  // namespace

void setup() {
  // Hold /WR and /RESET high before enabling their outputs.
  VPORTC.OUT |= PIN6_bm;
  VPORTC.DIR |= PIN6_bm;
  VPORTA.OUT |= PIN0_bm;
  VPORTA.DIR |= PIN0_bm;
  VPORTD.DIR |= 0x3f;
  VPORTB.DIR |= PIN0_bm | PIN1_bm;
  PORTF.PIN4CTRL = PORT_PULLUPEN_bm;  // Disconnected PBUSY reads busy.
  pinMode(7, INPUT_PULLUP);
  useSPI = digitalRead(7) == LOW;

  if (useSPI) {
    // ALT2: MOSI D11 (PE0), MISO D12 (PE1), SCK D13 (PE2), SS D8 (PE3).
    PORTE.DIRCLR = PIN0_bm | PIN2_bm | PIN3_bm;
    PORTE.DIRSET = PIN1_bm;
    PORTE.PIN3CTRL = PORT_PULLUPEN_bm;
    PORTMUX.TWISPIROUTEA = (PORTMUX.TWISPIROUTEA & ~PORTMUX_SPI0_gm) | PORTMUX_SPI0_ALT2_gc;
    SPI0.CTRLB = SPI_BUFEN_bm | SPI_BUFWR_bm | SPI_MODE_0_gc;
    SPI0.CTRLA = SPI_ENABLE_bm;  // Client, MSB first.
  } else {
    Serial.begin(usbBaud);
  }
}

void loop() {
  static uint8_t status;
  const int value = useSPI ? pollSPI(status) : Serial.read();
  if (value == startByte) {
    status = receiveRequest();
    if (!useSPI && status != 0) {
      Serial.write(status);
    }
    return;
  }
  servicePWM();
}
