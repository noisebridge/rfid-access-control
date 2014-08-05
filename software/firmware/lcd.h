/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#ifndef _AVR_LCD_H_
#define _AVR_LCD_H_

class LcdDisplay {
public:
  // Initialize LCD display with the given width.
  LcdDisplay(int width);

  // Print string into given row.
  void print(unsigned char row, const char *str);

private:
  const unsigned char width_;
};

#endif // _AVR_LCD_H_
