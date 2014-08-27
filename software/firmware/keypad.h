/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#ifndef AVR_KEYPAD_H_
#define AVR_KEYPAD_H_

#include "clock.h"

// For wiring more like 'pretty' physical layout, logical layout is a mess
// with bits distributed between ports :)
// row 1:PD7 2:PD6 3:PD5 4:PD4  (all port D; conveniently shifted by 4)
// col 1:PB0 2:PB7 3:PB6        (all port B)
class KeyPad {
  enum {
    ROW_PORTD_SHIFT = 4,  // We read from port-D to top 4 bits
    COL_1 = (1<<0),
    COL_2 = (1<<6),
    COL_3 = (1<<7),

    // columns will be outputs.
    PORTB_OUT_MASK = COL_1 | COL_2 | COL_3,
  };

public:
  KeyPad();

  // This method needs to be called regularly.
  // It returns the current character pressed on the keypad if it settled
  // for long enough (it returns it only once, no auto-repeat).
  // Returns a NUL character ('\0') if no key is pressed.
  char ReadKeypad();

private:
  typedef unsigned char keypad_state_t;
  keypad_state_t SampleCol(unsigned char col_sample_bit,
                           keypad_state_t col_data);
  keypad_state_t readKeypadState();

  keypad_state_t current_state_;
  bool character_returned_;
  Clock::cycle_t start_time_;
};

#endif  // AVR_KEYPAD_H_
