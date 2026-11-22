#include <Arduino.h>
#include "data/GU256X64D7000/src/GU256X64D7000.h"
#include "xmodem_crc.h"

#if !defined(ARDUINO_AVR_NANO_EVERY)
#error "Select Arduino Nano Every (arduino:megaavr:nona4809)"
#endif

namespace {
constexpr uint8_t startByte = 0xff;
constexpr uint8_t readyByte = 0xfe;
constexpr uint32_t usbBaud = 1000000;
constexpr uint8_t monoOp = 0x02;
constexpr uint8_t gray3Op = 0x03;
constexpr uint8_t gray2Op = 0x04;
constexpr uint8_t versionOp = 0x0f;
constexpr uint16_t protocolVersion = 0x0100;
constexpr uint8_t versionMajor = protocolVersion >> 8;
constexpr uint8_t versionMinor = protocolVersion & 0xff;
constexpr uint8_t selectMajor = 0x01;
constexpr uint8_t selectMinor = 0x02;
constexpr uint8_t finishVersion = 0x03;
constexpr uint8_t controlOpOffset = 0x10;
constexpr uint8_t vfdBitmapSubcommand = 0x11;  // US ( f
constexpr uint8_t vfdCursorCommand = 0x24;  // US $
constexpr uint8_t bitmapOp = vfdBitmapSubcommand + controlOpOffset;
constexpr uint8_t textOp = vfdCursorCommand + controlOpOffset;
constexpr uint8_t vfdPowerSubcommand = 0x40;  // US ( a
constexpr uint8_t vfdBrightnessCommand = 0x58;  // US X
constexpr uint8_t showOp = vfdPowerSubcommand + controlOpOffset;
constexpr uint8_t brightnessOp = vfdBrightnessCommand + controlOpOffset;
constexpr uint16_t monoSize = 256 * 64 / 8;
constexpr uint16_t width = 256;
constexpr uint8_t displayBands = 8;
constexpr uint8_t textCellWidth = 7;
constexpr uint8_t bitmapHeaderSize = 5;
constexpr uint8_t textHeaderSize = 2;
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

using VFD = gu256x64d7000::Display<>;
constexpr VFD vfd{};

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
  if (vfd.waitWhileBusy()) {
    return true;
  }
  vfdReady = false;
  return false;
}

inline bool waitVFD() {
  return !vfd.busy() || waitBusyVFD();
}

inline __attribute__((always_inline)) bool writeVFD(uint8_t value) {
  if (!waitVFD()) {
    return false;
  }
  vfd.writeReady(value, vfd.captureLatches());
  return true;
}

inline __attribute__((always_inline)) bool writeBitmapByte(uint8_t value, VFD::Latches latches) {
  if (!waitVFD()) {
    return false;
  }
  vfd.writeReady(value, latches);
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
  const bool written = vfd.setPower(enabled);
  vfdReady = vfdReady && written;
  return written;
}

bool setBrightness(uint8_t level) {
  const bool written = vfd.setBrightness(level);
  vfdReady = vfdReady && written;
  return written;
}

bool initVFD() {
  visibleBank = 0;
  pwmMode = monoOp;
  pwmPhase = 0;
  gray2FrameValid = false;
  gray2NextReady = false;
  vfdReady = vfd.initialize() &&
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
    const uint8_t *src, const uint8_t *mask, VFD::Latches latches) {
  const uint8_t a = mask[src[0]], b = mask[src[1]], c = mask[src[2]];
  const uint8_t d = mask[src[3]], e = mask[src[4]];
  // Five base-3 digits decode to five vertical mask bits. The first
  // five groups contribute 24 bits, with one bit carried into byte four.
  if (!writeBitmapByte((a << 3) | (b >> 2), latches) ||
      !writeBitmapByte((b << 6) | (c << 1) | (d >> 4), latches) ||
      !writeBitmapByte((d << 4) | (e >> 1), latches)) {
    return false;
  }
  if (!FullGroup) {
    return true;  // The 25th bit is padding in the last group of the column.
  }
  const uint8_t f = mask[src[5]], g = mask[src[6]], h = mask[src[7]];
  return writeBitmapByte((e << 7) | (f << 2) | (g >> 3), latches) &&
         writeBitmapByte((g << 5) | h, latches);
}

bool writeBank(uint8_t bank, Bitmap bitmap) {
  if (!command<15>({0x1f, vfdCursorCommand, 0, bank, 0, 0,  // Cursor (bank*256, 0).
                0x1f, 0x28, 0x66, vfdBitmapSubcommand, 0, 1, displayBands, 0, 1})) {
    return false;
  }
  // No other code writes these latches during the bitmap transfer.
  const auto latches = vfd.captureLatches();
  if (bitmap == Bitmap::Mono) {
    for (uint16_t i = 0; i < monoSize; ++i) {
      if (!writeBitmapByte(frame[i], latches)) {
        return false;
      }
    }
  } else if (bitmap == Bitmap::Gray3Lit || bitmap == Bitmap::Gray3Full) {
    const uint8_t *mask = bitmap == Bitmap::Gray3Full ? fullPixels : litPixels;
    for (const uint8_t *column = frame; column != frame + gray3Size; column += gray3Column) {
      if (!writeGray3Group<true>(column, mask, latches) ||
          !writeGray3Group<false>(column + 8, mask, latches)) {
        return false;
      }
    }
  } else {
    const uint8_t *mask = bitmap == Bitmap::Full ? fullPixels : litPixels;
    const uint8_t bits = bitmap == Bitmap::HighBit ? 0xaa : 0xff;
    for (uint16_t i = 0; i < gray2Size; i += 2) {
      if (!writeBitmapByte((mask[frame[i] & bits] << 4) | mask[frame[i + 1] & bits], latches)) {
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

bool drawNative(uint8_t op, const uint8_t *payload, uint16_t size) {
  const bool bitmap = op == bitmapOp;
  const uint8_t headerSize = bitmap ? bitmapHeaderSize : textHeaderSize;
  if (size <= headerSize || payload[1] >= displayBands) {
    return false;
  }

  const uint8_t x = payload[0], band = payload[1];
  const uint8_t *data = payload + headerSize;
  const uint16_t count = size - headerSize;
  uint16_t columns = 0;
  uint8_t bands = 0;
  if (bitmap) {
    columns = uint16_t(payload[3]) << 8 | payload[2];
    bands = payload[4];
    if (columns == 0 || columns > width - x || bands == 0 ||
        bands > displayBands - band || count != columns * bands) {
      return false;
    }
  } else {
    if (count > (width - x) / textCellWidth) {
      return false;
    }
    for (uint16_t i = 0; i < count; ++i) {
      if (data[i] < 0x20 || data[i] > 0x7e) {
        return false;
      }
    }
  }

  if ((!vfdReady && !initVFD()) ||
      !command<6>({0x1f, vfdCursorCommand, x, visibleBank, band, 0})) {
    return false;
  }
  if (bitmap && !command<9>({0x1f, 0x28, 0x66, vfdBitmapSubcommand,
                            uint8_t(columns), uint8_t(columns >> 8), bands, 0, 1})) {
    return false;
  }
  const auto latches = vfd.captureLatches();
  for (uint16_t i = 0; i < count; ++i) {
    if (!writeBitmapByte(data[i], latches)) {
      return false;
    }
  }
  return waitVFD();
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

bool receivePayload(uint8_t *payload, uint16_t size, uint32_t started) {
  for (uint16_t i = 0; i < size; ++i) {
    const int value = receiveByte(started);
    if (value < 0) {
      return false;
    }
    payload[i] = value;
  }
  return true;
}

bool receiveValidCRC(uint8_t op, uint8_t token, const uint8_t *payload, uint16_t size, uint32_t started) {
  uint16_t received = 0;
  for (uint8_t shift = 0; shift < 16; shift += 7) {
    const int value = receiveByte(started);
    if (value < 0 || value > (shift == 14 ? 3 : 127)) {
      return false;
    }
    received |= uint16_t(value) << shift;
  }
  return packetCRC(op, token, payload, size) == received;
}

bool receiveDrawing(uint8_t op, uint8_t token, uint32_t started) {
  const int low = receiveByte(started);
  const int high = receiveByte(started);
  if (low < 0 || high < 0) {
    return false;
  }
  const uint16_t size = uint16_t(high) << 8 | uint8_t(low);
  const uint16_t maxSize = op == bitmapOp ? bitmapHeaderSize + monoSize : textHeaderSize + width / textCellWidth;
  if (pwmMode != monoOp || size > maxSize) {
    // Drain the request without touching the packed grayscale frame.
    for (uint16_t i = 0; i < size; ++i) {
      if (receiveByte(started) < 0) {
        return false;
      }
    }
    for (uint8_t i = 0; i < 3; ++i) {
      if (receiveByte(started) < 0) {
        return false;
      }
    }
    return false;
  }

  frame[0] = low;
  frame[1] = high;
  return receivePayload(frame + 2, size, started) &&
         receiveValidCRC(op, token, frame, size + 2, started) &&
         drawNative(op, frame + 2, size);
}

uint8_t receiveRequest(bool &versionReply) {
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
  if (op == versionOp) {
    versionReply = receiveValidCRC(op, token, frame, 0, started);
    return versionReply ? token : reject;
  }
  if (op == bitmapOp || op == textOp) {
    return receiveDrawing(op, token, started) ? token : reject;
  }
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
  if (!receivePayload(payload, size, started) ||
      !receiveValidCRC(op, token, payload, size, started)) {
    return reject;
  }
  servicePWM();
  return execute(op, control) ? token : reject;
}
// Poll tightly during a version reply. Read the selector before queuing
// another byte so the remaining clocks flush any older transmit bytes.
uint8_t serveVersionSPI(uint8_t status) {
  const uint32_t started = millis();
  uint8_t spin = 0;
  for (;;) {
    if (SPI0.INTFLAGS & SPI_RXCIF_bm) {
      switch (SPI0.DATA) {
        case selectMajor:
          status = versionMajor;
          break;
        case selectMinor:
          status = versionMinor;
          break;
        case finishVersion:
          return readyByte;
        case startByte:
          return startByte;
      }
    }
    if (SPI0.INTFLAGS & SPI_DREIF_bm) {
      SPI0.DATA = status;
    }
    if (++spin == 0 && millis() - started >= 150) {
      return 0;
    }
  }
}
}  // namespace

void setup() {
  vfd.begin();
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
  if (value != startByte) {
    servicePWM();
    return;
  }

  do {
    bool versionReply = false;
    status = receiveRequest(versionReply);
    if (!useSPI) {
      if (status != 0) {
        Serial.write(status);
      }
      if (versionReply) {
        Serial.write(versionMajor);
        Serial.write(versionMinor);
      }
      return;
    }
    if (versionReply) {
      status = serveVersionSPI(status);
    }
  } while (status == startByte);
}
