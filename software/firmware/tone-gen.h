/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 *
 * Tone generator using 8-bit counter2 and output on a given port and PIN.
 * We already use the 16 bit counter for time-keeping and timer0 is too limited
 * to do arbitrary frequencies.
 *
 * The ToneGen::Tone() call is non-blocking: it returns immediately and
 * automatically turns off when the time is reached.
 */

#ifndef AVR_TONE_GEN_H_
#define AVR_TONE_GEN_H_

#include <stdint.h>

#include "clock.h"

class ToneGen {
#define TONE_GEN_OUT_PORT    PORTD
#define TONE_GEN_OUT_DATADIR DDRD
#define TONE_GEN_OUT_BIT     (1<<2)

public:
  ToneGen();

  static inline uint8_t hz_to_divider(uint16_t hz) {
    return (uint8_t) (F_CPU / 1024 / hz);
  }

  // Create tone for the given duration in cycles. The "divider" is best
  // determined from hz_to_divider()
  void Tone(uint8_t divider, Clock::cycle_t duration_cycles);
};
#endif  // AVR_TONE_GEN_H_
