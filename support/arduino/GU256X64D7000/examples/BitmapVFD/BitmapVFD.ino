#include <GU256X64D7000.h>

gu256x64d7000::Display<> vfd{};

constexpr uint8_t columns = 16;
constexpr uint8_t bands = 2;

// Two bytes per column, top band first. Bit 7 is the top pixel in each band.
const uint8_t bitmap[] = {
    0xff, 0xff, 0x80, 0x01, 0x80, 0x01, 0x80, 0x01,
    0x80, 0x01, 0x80, 0x01, 0x80, 0x01, 0x80, 0x01,
    0x80, 0x01, 0x80, 0x01, 0x80, 0x01, 0x80, 0x01,
    0x80, 0x01, 0x80, 0x01, 0x80, 0x01, 0xff, 0xff,
};
static_assert(sizeof(bitmap) == columns * bands, "Each column needs one byte per band");

void setup() {
  vfd.begin();
  vfd.reset();
  if (vfd.command<3>({0x1b, 0x40, 0x0c}) &&
      vfd.setCursor(120, 3) && vfd.beginBitmap(columns, bands)) {
    vfd.writeBytes(bitmap, sizeof(bitmap));
  }
}

void loop() {}
