/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */

#include "serial-com.h"

#include <avr/io.h>
#include <avr/interrupt.h>

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

SerialCom::SerialCom() : write_pos_(0), read_pos_(0), dropped_bytes_(0) {
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
  buffer[write_pos_] = c;
  unsigned char new_pos = (write_pos_ + 1) & ((1<<BUFFER_BITS)-1);
  if (new_pos != read_pos_)
    write_pos_ = new_pos;
  else
    ++dropped_bytes_;
}

unsigned char SerialCom::read_ready() {
  return (write_pos_ - read_pos_) & ((1<<BUFFER_BITS)-1);
}

char SerialCom::read() {
  while (!read_ready())
    ;
  char c = buffer[read_pos_];
  read_pos_ = (read_pos_ + 1) & ((1<<BUFFER_BITS)-1);
  return c;
}
