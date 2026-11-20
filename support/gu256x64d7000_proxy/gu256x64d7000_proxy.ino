#include <Arduino.h>
#include <util/crc16.h>

#if !defined(ARDUINO_AVR_NANO_EVERY)
#error "Select Arduino Nano Every (arduino:megaavr:nona4809)"
#endif

namespace {
constexpr uint8_t startByte = 0xff;
constexpr uint8_t readyByte = 0xfe;
constexpr uint8_t showOp = 2;
constexpr uint8_t brightnessOp = 3;
constexpr uint8_t gray3Op = 4;
constexpr uint8_t monoOp = 5;
constexpr uint8_t gray4Op = 6;
constexpr uint16_t monoSize = 256 * 64 / 8;
constexpr uint16_t graySize = 256 * 64 / 4;
constexpr uint8_t gray3PhaseMs[] = {14, 7, 7};
constexpr uint8_t gray4PhaseMs = 14;

uint8_t frame[graySize];
// Each lookup turns four packed pixels into four VFD bits. A high-bit
// mask on each input reuses litPixels for the >= 2 phase.
uint8_t litPixels[256];
uint8_t fullPixels[256];
enum class Bitmap : uint8_t { Mono, Lit, HighBit, Full };
constexpr Bitmap gray4Bitmaps[] = {Bitmap::Lit, Bitmap::HighBit, Bitmap::Full};

bool useSPI;
bool vfdReady;
bool visible = true;
uint8_t brightness = 8;
uint8_t visibleBank;
uint8_t pwmMode = monoOp;
uint8_t pwmPhase;
uint32_t phaseStarted;
bool gray4FrameValid;
bool gray4NextReady;

bool waitVFD() {
  // Most bytes are accepted quickly. Avoid running millis() on the fast path.
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

bool writeVFD(uint8_t value) {
  if (!waitVFD()) {
    return false;
  }
  // VFD D0..5 = PD0..5 and VFD D6..7 = PB0..1.
  VPORTD.OUT = (VPORTD.OUT & 0xc0) | (value & 0x3f);
  VPORTB.OUT = (VPORTB.OUT & 0xfc) | (value >> 6);
  VPORTC.OUT &= ~PIN6_bm;  // /WR (D4)
  asm volatile("nop\n\tnop" ::: "memory");
  VPORTC.OUT |= PIN6_bm;
  asm volatile("nop\n\tnop" ::: "memory");
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
  gray4FrameValid = false;
  gray4NextReady = false;
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

bool writeBank(uint8_t bank, Bitmap bitmap) {
  if (!command<15>({0x1f, 0x24, 0, bank, 0, 0,  // Cursor (bank*256, 0).
                0x1f, 0x28, 0x66, 0x11, 0, 1, 8, 0, 1})) {
    return false;
  }
  if (bitmap == Bitmap::Mono) {
    for (uint16_t i = 0; i < monoSize; ++i) {
      if (!writeVFD(frame[i])) {
        return false;
      }
    }
  } else {
    const uint8_t *mask = bitmap == Bitmap::Full ? fullPixels : litPixels;
    const uint8_t bits = bitmap == Bitmap::HighBit ? 0xaa : 0xff;
    for (uint16_t i = 0; i < graySize; i += 2) {
      if (!writeVFD((mask[frame[i] & bits] << 4) | mask[frame[i + 1] & bits])) {
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
  if (pwmMode == gray4Op) {
    if (!gray4FrameValid) {
      return;
    }
    if (!gray4NextReady) {
      // Reuse the hidden bank for the next of three distinct masks.
      gray4NextReady = writeBank(visibleBank ^ 1, gray4Bitmaps[next]);
      if (!gray4NextReady) {
        return;
      }
    }
  }

  const uint8_t duration = pwmMode == gray3Op ? gray3PhaseMs[pwmPhase] : gray4PhaseMs;
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
    gray4NextReady = false;
  }
}

bool showFrame(uint8_t op) {
  bool hasGray = false;
  if (op != monoOp) {
    for (uint16_t i = 0; i < graySize; ++i) {
      const uint8_t packed = frame[i];
      const uint8_t low = packed & 0x55;
      const uint8_t high = (packed >> 1) & 0x55;
      if (op == gray3Op && (low & high)) {
        return false;  // 11 is reserved in three-level mode.
      }
      hasGray |= op == gray3Op ? low != 0 : low != high;
    }
  }
  if (!vfdReady && !initVFD()) {
    return false;
  }

  const uint8_t mode = hasGray ? op : monoOp;
  uint8_t phase = 0;
  if (mode == pwmMode && hasGray) {
    phase = pwmPhase;
    if (mode == gray4Op && millis() - phaseStarted >= gray4PhaseMs) {
      phase = nextPhase();
    }
  }

  Bitmap bitmap = Bitmap::Mono;
  if (op == gray3Op) {
    bitmap = phase == 0 ? Bitmap::Lit : Bitmap::HighBit;
  } else if (op == gray4Op) {
    bitmap = gray4Bitmaps[phase];
  }
  if (!writeBank(visibleBank ^ 1, bitmap) || !switchBank()) {
    return false;
  }
  if (mode != pwmMode || phase != pwmPhase) {
    phaseStarted = millis();
  }
  if (mode == gray3Op && !writeBank(visibleBank ^ 1, phase == 0 ? Bitmap::HighBit : Bitmap::Lit)) {
    return false;
  }
  pwmMode = mode;
  pwmPhase = phase;
  gray4FrameValid = mode == gray4Op;
  gray4NextReady = false;
  return true;
}

bool execute(uint8_t op, uint8_t value) {
  if (op == monoOp || op == gray3Op || op == gray4Op) {
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
  while (millis() - started < 850) {
    int value = Serial.read();
    if (value >= 0) {
      return value;
    }
    servicePWM();
  }
  return -1;
}

uint8_t receiveRequest() {
  const uint32_t started = millis();
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
    op = receiveByte(started);
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
    case gray4Op:
      size = graySize;
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
    gray4FrameValid = false;
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
  uint16_t crc = _crc_xmodem_update(0, op);
  crc = _crc_xmodem_update(crc, token);
  for (uint16_t i = 0; i < size; ++i) {
    crc = _crc_xmodem_update(crc, payload[i]);
  }
  if (crc != received) {
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

  for (uint16_t packed = 0; packed < 256; ++packed) {
    uint8_t value = packed;
    for (uint8_t bit = 8; bit; bit >>= 1, value >>= 2) {
      const uint8_t level = value & 3;
      if (level >= 1) {
        litPixels[packed] |= bit;
      }
      if (level == 3) {
        fullPixels[packed] |= bit;
      }
    }
  }

  if (useSPI) {
    // ALT2: MOSI D11 (PE0), MISO D12 (PE1), SCK D13 (PE2), SS D8 (PE3).
    PORTE.DIRCLR = PIN0_bm | PIN2_bm | PIN3_bm;
    PORTE.DIRSET = PIN1_bm;
    PORTE.PIN3CTRL = PORT_PULLUPEN_bm;
    PORTMUX.TWISPIROUTEA = (PORTMUX.TWISPIROUTEA & ~PORTMUX_SPI0_gm) | PORTMUX_SPI0_ALT2_gc;
    SPI0.CTRLB = SPI_BUFEN_bm | SPI_BUFWR_bm | SPI_MODE_0_gc;
    SPI0.CTRLA = SPI_ENABLE_bm;  // Client, MSB first.
  } else {
    Serial.begin(115200);
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
