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

namespace ToneGen {
// The Pin we want to output the tone to. Can be an arbitrary port.
#define TONE_GEN_OUT_PORT    PORTD
#define TONE_GEN_OUT_DATADIR DDRD
#define TONE_GEN_OUT_BIT     (1<<2)

// Init ToneGen module.
void Init();

// Converts frequenzy Hz to necessary divider to be passed into Tone().
// If you do that at compile-time, it uses less code-space.
// The resulting frequency is only very rough, don't assume musical-note
// spot-on-ness (we don't have enough resolution of this).
inline uint8_t hz_to_divider(uint16_t hz) {
  return (uint8_t) (F_CPU / 1024 / hz);
}

// Create tone for the given duration in cycles. The "divider" divides the
// internal frequency to get the output. If you want get something related
// to more understandable Hz, creaate this with hz_to_divider().
inline void ToneOn(uint8_t divider) {
  OCR2 = divider;
  TIMSK |= (1<<OCIE2);   // switch on interrupt
}

// Switch off note started with ToneOn()
inline void ToneOff() {
  TIMSK &= ~(1<<OCIE2); // disable interrupt.
}

// Creates tone with the given "divider" (see ToneOn())
// The call is asynchronous: it returns immediately and runs for the given
// number of cycles, free time for you to do something else :)
void Tone(uint8_t divider, Clock::cycle_t duration_cycles);
}  // namespace ToneGen
#endif  // AVR_TONE_GEN_H_
