/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*- */

#include <avr/io.h>
#include <string.h>
#include <util/delay.h>
#include <stdio.h>   // snprintf

#include "mfrc522.h"

#define AUX_PORT PORTC
#define AUX_BITS 0x3F

class SerialComm {
public:
  // 9600 baud, 8 bit, no parity, 1 stop
  SerialComm() {
    const unsigned int divider = (F_CPU  / 17 / SERIAL_BAUDRATE) - 1;
    UBRRH = (unsigned char)(divider >> 8);
    UBRRL = (unsigned char) divider;
    UCSRB = (1<<RXEN) | (1<<TXEN);  // read and write
    UCSRC = (1<<URSEL) /*write-reg*/ | (1<<UCSZ1) | (1<<UCSZ0); /*8bit*/
  }

  void write(char c) {
    while ( !( UCSRA & (1<<UDRE)) )  // wait for transmit buffer to be ready.
      ;
    UDR = c;
  }

  // Convenience method.
  void writeString(const char *str) {
    while (*str) write(*str++);
  }

  // Convenience method.
  void writeHex(unsigned char c) {
    write(to_hex(c >> 4));
    write(to_hex(c & 0x0f));
  }

  inline bool read_ready() {
    return (UCSRA & (1<<RXC));
  }

  char read() {
    while (!read_ready())
      ;
    return UDR;
  }
private:
  char to_hex(unsigned char c) { return c < 0x0a ? c + '0' : c + 'a' - 10; }
};

// A line buffer reading from the serial communication line. Provides a
// nonblocking way to fill a buffer.
class LineBuffer {
public:
  LineBuffer() : pos_(buffer_) { }

  // Empties serial input buffer and stores in internal buffer.
  // Returns number of characters if newline reached or buffer full.
  // Returns '0' while this condition is not yet reached.
  byte noblockReadline(SerialComm *comm) {
    const char *end_buf = buffer_ + sizeof(buffer_) - 1;
    bool newline_seen = false;
    while (!newline_seen && comm->read_ready() && pos_ < end_buf) {
      const char c = (*pos_++ = comm->read());
      newline_seen = (c == '\r' || c == '\n');
    }
    *pos_ = '\0';  // We always have at least one byte space.
    if (newline_seen || pos_ >= end_buf) {
      byte len = pos_ - buffer_;
      pos_ = buffer_;
      return len;
    } else {
      return 0;
    }
  }

  // Returns current line, '\0' terminated.
  const char *line() const { return buffer_; }

private:
  char buffer_[32 + 1];
  char *pos_;
};

static void printHelp(SerialComm *out) {
  // Keep short or memory explodes :)
  out->writeString(
    "? Noisebridge RFID outpost | v0.1 | 8/2014\r\n"
    "? Sends:\r\n"
    "? R <num-bytes-hex> <uid-hex-str>\r\n"
    "? Commands:\r\n"
    "?\t?      This help\r\n"
    "?\tP      Ping\r\n"
    "?\tr      Reset reader\r\n"
    "?\tS<xx>  Set output bits; param 8bit hex\r\n");
}

static void setAuxBits(const char *buffer, SerialComm *out) {
  int value;
  if (1 == sscanf(buffer, "S%x", &value)) {
    value &= AUX_BITS;
    PORTC = value;
    char buf[8];
    snprintf(buf, sizeof(buf), "S%02x\r\n", value);
    out->writeString(buf);
  } else {
    out->writeString("S<invalid>\r\n");
  }
}

static void writeUid(const MFRC522::Uid &uid, SerialComm *out) {
  if (uid.size > 15) return;  // fishy.
  out->writeString("R ");
  out->writeHex((unsigned char) uid.size);
  out->write(' ');
  for (int i = 0; i < uid.size; ++i) {
    out->writeHex(uid.uidByte[i]);
  }
  out->writeString("\r\n");
}

int main() {
  DDRC = AUX_BITS;

  _delay_ms(100);  // Wait for voltage to settle before we reset the 522

  MFRC522 card_reader;
  card_reader.PCD_Init();

  MFRC522::Uid current_uid;

  SerialComm comm;
  LineBuffer lineBuffer;
  comm.writeString("Noisebridge access control outpost. '?' for help.\r\n");
  int rate_limit = 0;

  for (;;) {
    // See if there is a command incoming.
    if (lineBuffer.noblockReadline(&comm) != 0) {
      switch (lineBuffer.line()[0]) {
      case '?':
        printHelp(&comm);
        break;
      case 'P':
        comm.writeString("Pong\r\n");
        break;
      case 'S':
        setAuxBits(lineBuffer.line(), &comm);
        break;
      case 'r':
        card_reader.PCD_Reset();
        card_reader.PCD_Init();
        current_uid.size = 0;
        comm.writeString("reset RFID reader.\r\n");
        break;
      case '\r': case '\n':
        break;  // ignore spurious newline.
      default:
        comm.writeString("? Unknown command; '?' for help.\r\n");
      }
    }

    // ... or some new card found.
    if (!card_reader.PICC_IsNewCardPresent())
      continue;
    if (!card_reader.PICC_ReadCardSerial()) {
      current_uid.size = 0;
      continue;
    }
    if (--rate_limit > 0
        && current_uid.size == card_reader.uid.size
        && memcmp(current_uid.uidByte, card_reader.uid.uidByte,
                  current_uid.size) == 0)
      continue;
    rate_limit = 10;
    current_uid = card_reader.uid;
    writeUid(current_uid, &comm);
  }
}
