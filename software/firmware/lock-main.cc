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
  void writeHex(char c) {
    write(to_hex[(unsigned char) c >> 4]);
    write(to_hex[(unsigned char) c & 0x0f]);
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
  static const char to_hex[16];
};
const char SerialComm::to_hex[] = { '0','1','2','3','4','5','6','7',
                                    '8','9','a','b','c','d','e','f' };

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
  out->writeString(
    "? Noisebridge RFID reader protocol v0.1 8/2014\r\n"
    "? Continuously monitors keypad and RFID. Whenever one of these are read,\r\n"
    "? sends out a single line, prefixed with 'K' or 'R' followed by the content:\r\n"
    "? K <digits-typed-in-keypad-ending-with-#>\r\n"
    "?   or\r\n"
    "? R <number-of-bytes-hex> <serial-id-from-rfid-as-hex-string>\r\n"
    "? (Note: keypad not implemented yet)\r\n"
    "? The prototocol understands a few commands. They are all one-line commands\r\n"
    "? starting with a single command character. The response can be\r\n"
    "? one or multiple lines, prefixed with the command character.\r\n"
    "? Commands:\r\n"
    "? \t?      This help.\r\n"
    "? \tP      Ping (to check for aliveness).\r\n"
    "? \tS<xx>  Set output bits. <xx> is an 8-bit hex number.\r\n");
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

static void handleCommand(const char *buffer, SerialComm *out) {
  switch (buffer[0]) {
  case '?':
    printHelp(out);
    break;
  case 'P':
    out->writeString("Pong\r\n");
    break;
  case 'S':
    setAuxBits(buffer, out);
    break;
  case '\r': case '\n':
    break;  // ignore spurious newline.
  default:
    out->writeString("? Unknown command. Try '?' for help.\r\n");
  }
}

static void writeUid(const MFRC522::Uid &uid, SerialComm *out) {
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

  MFRC522 card_reader;
  card_reader.PCD_Init();

  MFRC522::Uid current_uid;

  SerialComm comm;
  LineBuffer lineBuffer;
  comm.writeString("Noisebridge access control outpost. '?' for help.\r\n");
  int rate_limit = 0;

  for (;;) {
    if (lineBuffer.noblockReadline(&comm) != 0)
      handleCommand(lineBuffer.line(), &comm);
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
