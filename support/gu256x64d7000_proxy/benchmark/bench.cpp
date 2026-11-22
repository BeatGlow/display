#include <util/crc16.h>

#include "../gu256x64d7000_proxy.ino"
#include "fixture.h"

volatile uint8_t benchmarkOp;
volatile uint8_t benchmarkOK;
volatile uint16_t benchmarkChecksum;

extern "C" {
#define MARK __attribute__((noinline, noclone, used))
void MARK benchStart() { asm volatile("nop" ::: "memory"); }
void MARK benchCRCComplete() { asm volatile("nop" ::: "memory"); }
void MARK benchPublished() { asm volatile("nop" ::: "memory"); }
void MARK benchEnd() { asm volatile("nop" ::: "memory"); }
#undef MARK
}

int main() {
  setup();
  PORTF.PIN4CTRL = 0;  // Simulate PBUSY continuously ready.
  vfdReady = true;

  const uint8_t formats[] = {monoOp, gray3Op, gray2Op};
  const uint8_t *fixtures[] = {monoFixture, gray3Fixture, gray2Fixture};
  const uint16_t sizes[] = {sizeof monoFixture, sizeof gray3Fixture, sizeof gray2Fixture};
  for (uint8_t i = 0; i < 3; ++i) {
    const uint8_t op = formats[i];
    const uint16_t size = sizes[i];
    benchmarkOp = op;
    uint16_t reference = _crc_xmodem_update(_crc_xmodem_update(0, op), 1);
    for (uint16_t j = 0; j < size; ++j) {
      frame[j] = pgm_read_byte(fixtures[i] + j);
      reference = _crc_xmodem_update(reference, frame[j]);
    }
    if (op != monoOp) {
      prepareMasks(op);
    }
    if (op == gray3Op) {
      const uint8_t first = frame[0];
      frame[0] = 243;
      const bool rejectsDigit = !showFrame(op);
      frame[0] = first;
      const uint8_t last = frame[gray3Column - 1];
      frame[gray3Column - 1] = 81;
      const bool rejectsPadding = !showFrame(op);
      frame[gray3Column - 1] = last;
      if (!rejectsDigit || !rejectsPadding) {
        for (;;) {}
      }
    }

    benchStart();
    benchmarkChecksum = packetCRC(op, 1, frame, size);
    benchCRCComplete();
    const bool published = showFrame(op);
    benchPublished();
    servicePWM();
    benchmarkOK = published && benchmarkChecksum == reference &&
                  (op != gray2Op || gray2NextReady);
    benchEnd();
  }
  for (;;) {}
}
