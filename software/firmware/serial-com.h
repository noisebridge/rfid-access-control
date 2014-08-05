/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#ifndef _AVR_SERIAL_H_
#define _AVR_SERIAL_H_

// Since we can't really do memory allocation but want a configurable
// size, we'll just use a template.
// Meant to be used one-sided in an interrupt handler, so everything is volatile.
template<int BUFFER_BITS> class RingBuffer {
public:
  RingBuffer();

  // Number of spots available to write.
  unsigned char write_ready() volatile;

  // Write. Blocks until write_ready()
  void write(char c) volatile;

  // Number of bytes ready to read.
  unsigned char read_ready() volatile;

  // Read a byte. Blocks if read_ready() == 0.
  char read() volatile;

private:
  volatile unsigned char write_pos_;
  volatile unsigned char read_pos_;
  char buffer_[1<<BUFFER_BITS];
};

class SerialCom {
  enum { RX_BUFFER_BITS = 5 };  // Buffer uses 2^BUFFER_BITS bytes.
public:
  // Setting up the serial interface. Baudrate is given as -DSERIAL_BAUDRATE
  // Otherwise simply 8N1.
  // Also assumes -DF_CPU to be set.
  // Internally maintains an incoming buffer with 2^BUFFER_BITS size.
  SerialCom();

  // Write a single character. Blocks if buffer full.
  void write(char c);

  // Bytes ready to read. Can be up to buffer-size.
  unsigned char read_ready() volatile;

  // Read a single chracter. Blocks if nothing in buffer.
  char read() volatile;

  // Number of bytes we dropped reading.
  unsigned short dropped_reads() const { return dropped_reads_; }

private:
  friend class SerialComISRWriter;
  void StuffByte(char c) volatile;  // Stuff into buffer. Called by ISR.
  unsigned short dropped_reads_;

  RingBuffer<RX_BUFFER_BITS> read_buffer_;
  // TODO: ring buffer for writing.
};

#endif  // _AVR_SERIAL_H_
