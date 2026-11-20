#include <Arduino.h>
#include <util/crc16.h>

#if !defined(ARDUINO_AVR_NANO_EVERY)
#error "Select Arduino Nano Every (arduino:megaavr:nona4809)"
#endif

namespace {
constexpr uint8_t startByte = 0xff;
constexpr uint8_t readyByte = 0xfe;
constexpr uint8_t frameOp = 1;
constexpr uint8_t showOp = 2;
constexpr uint8_t brightnessOp = 3;
constexpr uint16_t width = 256;
constexpr uint8_t groupsPerColumn = 13;
constexpr uint16_t frameSize = width * groupsPerColumn;
constexpr uint8_t pwmPhaseMs = 14;

// Keep the two display masks in the VFD's own 512-column RAM.
uint8_t frame[frameSize];
uint8_t anyMask[243];
uint8_t fullMask[243];

bool useSPI;
bool vfdReady;
bool visible = true;
bool grayscale;
uint8_t brightness = 8;
uint8_t visibleBank;
uint32_t phaseStarted;

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
  grayscale = false;
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
  phaseStarted = millis();
  return true;
}

void servicePWM() {
  if (vfdReady && visible && grayscale && millis() - phaseStarted >= pwmPhaseMs) {
    switchBank();
  }
}

bool writeBank(uint8_t bank, const uint8_t *mask) {
  if (!command<15>({0x1f, 0x24, 0, bank, 0, 0,  // Cursor (bank*256, 0).
                0x1f, 0x28, 0x66, 0x11, 0, 1, 8, 0, 1})) {
    return false;
  }
  const uint8_t *column = frame;
  for (uint16_t x = 0; x < width; ++x, column += groupsPerColumn) {
    uint16_t bits = 0;
    uint8_t pending = 0;
    for (uint8_t group = 0; group < groupsPerColumn; ++group) {
      bits = (bits << 5) | mask[column[group]];
      pending += 5;
      if (pending >= 8) {
        pending -= 8;
        if (!writeVFD(uint8_t(bits >> pending))) {
          return false;
        }
      }
    }
  }
  return waitVFD();
}

bool showFrame() {
  bool hasGray = false;
  for (uint16_t i = 0; i < frameSize; ++i) {
    const uint8_t packed = frame[i];
    // Each column ends with four pixels and a zero padding trit.
    if (packed >= sizeof(anyMask) || (i % groupsPerColumn == groupsPerColumn - 1 && packed >= 81)) {
      return false;
    }
    hasGray |= anyMask[packed] != fullMask[packed];
  }
  if (!vfdReady && !initVFD()) {
    return false;
  }
  grayscale = false;
  if (!writeBank(visibleBank ^ 1, anyMask) || !switchBank()) {
    return false;
  }
  if (hasGray && !writeBank(visibleBank ^ 1, fullMask)) {
    return false;
  }
  grayscale = hasGray;
  return true;
}

bool execute(uint8_t op, uint8_t value) {
  if (op == frameOp) {
    return showFrame();
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
  if (op != frameOp && op != showOp && op != brightnessOp) {
    return reject;
  }

  const uint16_t size = op == frameOp ? frameSize : 1;
  for (uint16_t i = 0; i < size; ++i) {
    int value = receiveByte(started);
    if (value < 0 || value >= readyByte) {
      return reject;
    }
    frame[i] = value;
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
    crc = _crc_xmodem_update(crc, frame[i]);
  }
  if (crc != received) {
    return reject;
  }
  servicePWM();
  return execute(op, frame[0]) ? token : reject;
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

  for (uint8_t packed = 0; packed < sizeof(anyMask); ++packed) {
    uint8_t value = packed;
    for (uint8_t bit = 0x10; bit; bit >>= 1) {
      const uint8_t trit = value % 3;
      value /= 3;
      if (trit != 0) {
        anyMask[packed] |= bit;
      }
      if (trit == 2) {
        fullMask[packed] |= bit;
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
  }
  servicePWM();
}
