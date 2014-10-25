/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#include "tone-gen.h"

#include "clock.h"

#include <avr/io.h>
#include <avr/interrupt.h>
#include <util/delay.h>

ToneGen::ToneGen() {
    TONE_GEN_OUT_DATADIR |= TONE_GEN_OUT_BIT;
    TCCR2= (1<<CS22) | (1<<CS21) | (1<<CS20)   // clk/1024
      | (1<<WGM21);                // OCR2 compare.
}

static volatile Clock::cycle_t wait_time_;
static volatile Clock::cycle_t start_time_;

// The pins for output compare are already in other good use (SPI), so we need
// to manually toggle the output of a separate pin.
ISR(TIMER2_COMP_vect) {
  if (Clock::now() - start_time_ < wait_time_) {
    TONE_GEN_OUT_PORT ^= TONE_GEN_OUT_BIT;
  } else {
    TIMSK &= ~(1<<OCIE2); // disable interrupt.
  }
}

void ToneGen::Tone(uint8_t divider, Clock::cycle_t duration_cycles) {
  start_time_ = Clock::now();
  wait_time_ = duration_cycles;
  OCR2 = divider;
  TIMSK |= (1<<OCIE2);   // switch on interrupt
}

