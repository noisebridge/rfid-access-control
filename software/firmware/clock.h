/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#ifndef AVR_CLOCK_H_
#define AVR_CLOCK_H_

#include <avr/io.h>

/* A 'clock' interface. To do comparisons, it is cheapest to compare with
 * a constant evaluated number of cycles instead of converting cycles to seconds
 * every time. So do idioms like:
 *  if (Clock::now() - last_cyles < Clock::ms_to_cycles(20)) {}
 * This should be simply a namespace instad of a class, but then the compiler
 * warns about static functions being defined and not used.
 */
class Clock {
public:
  typedef unsigned short cycle_t;

  static void init() {
    TCCR1B = (1<<CS12) | (1<<CS10);  // clk/1024
  }

  // The timer with aroud 7.8 kHz rolls over the 64k every 8.3 seconds: it
  // makes only sense to do unsigned time comparisons <= 8.3 seconds.
  // Returns clock ticks.
  static cycle_t now() { return TCNT1; }

  // Converts milliseconds into clock cycles. If you provide a constant
  // expression, the compiler will be able to replace this with a constant,
  // otherwise it'll get expensive (division and such).
  static cycle_t ms_to_cycles(unsigned short ms) {
    return ms * (F_CPU / 1024/*prescaler*/) / 1000/*ms*/;
  }
};

#endif  // AVR_CLOCK_H_
