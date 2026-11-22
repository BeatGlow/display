#pragma once

#include <avr/io.h>
#include <stddef.h>
#include <stdint.h>

// The benchmark exercises the production GPIO and frame code with no
// Arduino interrupts or attached UART. Time stays in the current PWM phase.
constexpr uint8_t LOW = 0;
constexpr uint8_t HIGH = 1;
constexpr uint8_t OUTPUT = 1;
constexpr uint8_t INPUT_PULLUP = 2;
inline uint32_t millis() { return 0; }
inline void delay(uint32_t) {}
inline void delayMicroseconds(unsigned int) {}
inline void pinMode(uint8_t, uint8_t) {}
inline uint8_t digitalRead(uint8_t) { return HIGH; }
inline void digitalWrite(uint8_t, uint8_t) {}

struct BenchmarkSerial {
  void begin(uint32_t) {}
  int read() { return -1; }
  void write(uint8_t) {}
};

static BenchmarkSerial Serial;
