/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#include "keypad.h"
#include <util/delay.h>

// Long enough to not have bounce effects, but short enough to have snappy
// keypress responses.
#define DEBOUNCE_TIME_MILLIS 50

KeyPad::KeyPad() : current_state_(0), character_returned_(true) {
  DDRB |= PORTB_OUT_MASK;
  DDRD &= 0x0f;
  PORTD |= 0xf0;  // pull-up
}

char KeyPad::ReadKeypad() {
  const keypad_state_t state = readKeypadState();
  if (state != current_state_) {  // Change! Prepare to wait for it to settle.
    character_returned_ = false;
    current_state_ = state;
    start_time_ = Clock::now();
    return 0;
  }
  if (character_returned_)
    return 0;  // we already returned it.

  if (Clock::now() - start_time_ < Clock::ms_to_cycles(DEBOUNCE_TIME_MILLIS))
    return 0;  // not yet valid long enough.

  character_returned_ = true;
  // Columns are encoded in the higher nibble, rows in the lower one.
  switch (state) {
  case 0b0010001: return '1';  // first row, first col.
  case 0b0100001: return '2';
  case 0b1000001: return '3';

  case 0b0010010: return '4';  // second row
  case 0b0100010: return '5';
  case 0b1000010: return '6';

  case 0b0010100: return '7';  // third row
  case 0b0100100: return '8';
  case 0b1000100: return '9';

  case 0b0011000: return '*';  // fourth row
  case 0b0101000: return '0';
  case 0b1001000: return '#';

  default:
    return '\0';  // someone pressing the keys in a wierd way.
  }
}

KeyPad::keypad_state_t KeyPad::SampleCol(unsigned char col_sample_bit,
                                         keypad_state_t col_data) {
  PORTB |= PORTB_OUT_MASK;
  PORTB &= ~col_sample_bit;  // We sample active low.
  _delay_ms(1);  // Limit sampling rate on crusty high inductance connections.
  const keypad_state_t row_data = (PIND >> ROW_PORTD_SHIFT) ^ 0x0f;
  return row_data ? (col_data | row_data) : 0;
}

KeyPad::keypad_state_t KeyPad::readKeypadState() {
  keypad_state_t result = 0;
  // We collect everything in one convenient byte. We don't worry at all here if
  // this is a valid keypress. That happens when we consider it in the switch.
  result |= SampleCol(COL_1, 1<<4);
  result |= SampleCol(COL_2, 1<<5);
  result |= SampleCol(COL_3, 1<<6);
  return result;
}
