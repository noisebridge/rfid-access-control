/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#include "serial-com.h"

#include <avr/io.h>
#include <avr/interrupt.h>

template<int BUFFER_BITS> RingBuffer<BUFFER_BITS>::RingBuffer()
  : write_pos_(0), read_pos_(0) {
}

template<int BUFFER_BITS> unsigned char
RingBuffer<BUFFER_BITS>::write_available() volatile {
  return (read_pos_ - (write_pos_ + 1)) & MODULO_MASK;
}

template<int BUFFER_BITS> void RingBuffer<BUFFER_BITS>::write(char c) volatile {
  while (!write_available())
    ;
  buffer_[write_pos_] = c;
  write_pos_ = (write_pos_ + 1) & MODULO_MASK;
}

template<int BUFFER_BITS> unsigned char
RingBuffer<BUFFER_BITS>::read_available() volatile {
  return (write_pos_ - read_pos_) & MODULO_MASK;
}

template<int BUFFER_BITS> char RingBuffer<BUFFER_BITS>::read() volatile {
  while (!read_available())
    ;
  char c = buffer_[read_pos_];
  read_pos_ = (read_pos_ + 1) & MODULO_MASK;
  return c;
}

static volatile SerialCom *global_ser = 0; // ISR needs to access the Serial.

// Work around private visibility.
class SerialComISRWriter {
public:
  static void StuffByte(char c) {
    global_ser->StuffByte(c);
  }
};

ISR(USART_RXC_vect) {
  if (UCSRA & (1<<RXC)) {
    SerialComISRWriter::StuffByte(UDR);
  }
}

SerialCom::SerialCom() : dropped_reads_(0) {
  global_ser = this;  // our ISR needs access.
  const unsigned int divider = (F_CPU  / 17 / SERIAL_BAUDRATE) - 1;
  UBRRH = (unsigned char)(divider >> 8);
  UBRRL = (unsigned char) divider;
  UCSRB = (1<<RXCIE) | (1<<RXEN) | (1<<TXEN);  // read and write; interrupt read
  UCSRC = (1<<URSEL) /*write-reg*/ | (1<<UCSZ1) | (1<<UCSZ0); /*8bit*/
  sei();  // Enable interrupts.
}

void SerialCom::write(char c) {
  while ( !( UCSRA & (1<<UDRE)) )  // wait for transmit buffer to be ready.
    ;
  UDR = c;
}

void SerialCom::StuffByte(char c) volatile {
  if (rx_buffer_.write_available())
    rx_buffer_.write(c);
  else
    ++dropped_reads_;
}

unsigned char SerialCom::read_available() volatile {
  return rx_buffer_.read_available();
}

char SerialCom::read() volatile {
  return rx_buffer_.read();
}
