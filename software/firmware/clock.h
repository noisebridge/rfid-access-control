/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#ifndef AVR_CLOCK_H_
#define AVR_CLOCK_H_

#include <avr/io.h>
#include <stdint.h>

/* A 'clock' interface using the 16-Bit counter1.
 * To do comparisons, it is cheapest to compare with
 * a compile-time constant evaluated number of cycles instead of converting
 * cycles to seconds every time.
 * So do idioms like:
 *    if (Clock::now() - last_cyles < Clock::ms_to_cycles(20)) {}
 * The counter rolls over every 8388ms so only time comparisons up to that
 * value make sense.
 */
namespace Clock {
  typedef uint16_t cycle_t;

  static inline void init() {
    TCCR1B = (1<<CS12) | (1<<CS10);  // clk/1024
  }

  // The timer with aroud 7.8 kHz rolls over the 64k every 8.3 seconds: so it
  // makes only sense to do unsigned time comparisons <= 8.3 seconds.
  // Returns clock ticks.
  static inline cycle_t now() { return TCNT1; }

  // Converts milliseconds into clock cycles. If you provide a constant
  // expression at compile-time, the compiler will be able to replace this
  // with a constant, otherwise it'll get expensive (division and such).
  static inline cycle_t ms_to_cycles(uint16_t ms) {
    return ms * (F_CPU / 1024/*prescaler*/) / 1000/*ms*/;
  }
};

#endif  // AVR_CLOCK_H_
