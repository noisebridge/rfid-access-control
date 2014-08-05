/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#include "lcd.h"

#include <avr/io.h>
#include <util/delay.h>

// Somewhat hardcoded: we use PORTC, the lower nibble for data, 2 bits for conrol
#define LCD_PORT   PORTC
#define LCD_DDR    DDRC
#define LCD_BITS   0x3F

#define BIT_RS     0x10
#define BIT_ENABLE 0x20

// According to datasheet, common operations take up to ~37usec
#define LCD_DISPLAY_OPERATION_WAIT_USEC 50

static void WriteNibble(bool is_command, unsigned char b) {
  LCD_PORT = b & 0x0f;
  LCD_PORT |= (is_command ? 0 : BIT_RS) | BIT_ENABLE;
  for (int i = 0; i < 10; ++i) {}
  LCD_PORT &= ~BIT_ENABLE;
}
static void WriteByte(bool is_command, unsigned char b) {
  WriteNibble(is_command, (b >> 4) & 0xf);
  WriteNibble(is_command, b & 0xf);
  _delay_us(LCD_DISPLAY_OPERATION_WAIT_USEC);
}

LcdDisplay::LcdDisplay(int width) : width_(width) {
  DDRC = LCD_BITS;

  // -- This seems to be a reliable initialization sequence:

  // Start with 8 bit mode, then instruct to switch to 4 bit mode.
  WriteNibble(true, 0x03);
  _delay_us(5000);       // If we were in 4 bit mode, timeout makes this 0x30
  WriteNibble(true, 0x03);
  _delay_us(5000);

  // Transition to 4 bit mode.
  WriteNibble(true, 0x02); // Interpreted as 0x20: 8-bit cmd: switch to 4-bit.
  _delay_us(LCD_DISPLAY_OPERATION_WAIT_USEC);

  // From now on, we can write full bytes that we transfer in nibbles.
  WriteByte(true, 0x28);  // Function set: 4-bit mode, two lines, 5x8 font
  WriteByte(true, 0x06);  // Entry mode: increment, no shift
  WriteByte(true, 0x0c);  // Display control: on, no cursor

  WriteByte(true, 0x01);  // Clear display
  _delay_us(2000);        // ... which takes up to 1.6ms
}

void LcdDisplay::print(unsigned char row, const char *str) {
  if (row > 1) return;
  // Set address to write to; line 2 starts at 0x40
  WriteByte(true, 0x80 + ((row > 0) ? 0x40 : 0));
  unsigned char pos;
  for (pos = 0; *str && pos < width_; str++, pos++) {
    WriteByte(false, *str);
  }
  for (/**/; pos < width_; ++pos) {
    WriteByte(false, ' ');  // fill rest with spaces.
  }
}
