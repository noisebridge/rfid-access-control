/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#ifndef _AVR_SERIAL_H_
#define _AVR_SERIAL_H_

class SerialCom {
  enum { BUFFER_BITS = 6 };  // Buffer uses 2^BUFFER_BITS bytes.
public:
  // Setting up the serial interface. Baudrate is given as -DSERIAL_BAUDRATE
  // Otherwise simply 8N1.
  // Also assumes -DF_CPU to be set.
  // Internally maintains an incoming buffer with 2^BUFFER_BITS size.
  SerialCom();

  // Write a single character. Blocks if buffer full.
  void write(char c);

  // Bytes ready to read. Can be up to buffer-size.
  unsigned char read_ready();

  // Read a single chracter. Blocks if nothing in buffer.
  char read();

  // Number of bytes we dropped reading.
  unsigned short dropped_bytes() const volatile { return dropped_bytes_; }

private:
  friend class SerialComISRWriter;
  void StuffByte(char c) volatile;  // Stuff into buffer. Called by ISR.

  volatile unsigned char write_pos_;
  unsigned char read_pos_;
  volatile unsigned short dropped_bytes_;
  char buffer[1<<BUFFER_BITS];  // We want this a power of two for cheap modulo
};

#endif  // _AVR_SERIAL_H_
