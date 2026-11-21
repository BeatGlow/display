#include <GU256X64D7000.h>

using namespace gu256x64d7000;

#if defined(ARDUINO_AVR_UNO_WIFI_REV2)
// Uno WiFi Rev2: A0..A5 and D9..D10 for data, D3 for PBUSY,
// D5 for /WR, and D7 for /RESET.
using BoardWiring = Wiring<SplitData<Port::D, 0, 6, Port::B, 0>,
                           Pin<Port::F, 5>, Pin<Port::B, 2>, Pin<Port::A, 1>>;
Display<BoardWiring> vfd{BoardWiring{}};
#else
Display<> vfd{};
#endif

void setup() {
  vfd.begin();
  vfd.reset();
  if (vfd.command<3>({0x1b, 0x40, 0x0c}) && vfd.setCursor(0, 0)) {
    const uint8_t text[] = {'H', 'e', 'l', 'l', 'o'};
    vfd.writeBytes(text, sizeof(text));
  }
}

void loop() {}
