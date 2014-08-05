/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 */
#include <avr/io.h>
#include <string.h>
#include <util/delay.h>

#include "mfrc522.h"
#include "lcd.h"
#include "serial-com.h"

#define AUX_PORT PORTC
#define AUX_BITS 0x3F

static char to_hex(unsigned char c) { return c < 0x0a ? c + '0' : c + 'a' - 10; }

// returns value 0x00..0x0f or 0xff for failure.
static unsigned char from_hex(unsigned char c) {
  if (c >= '0' && c <= '9') return c - '0';
  if (c >= 'a' && c <= 'f') return c - 'a' + 10;
  if (c >= 'A' && c <= 'F') return c - 'A' + 10;
  return 0xff;
}

static inline bool isWhitespace(char c) { return c == ' ' || c == '\t'; }

// Skips whitespace, reads the last available two digits into result. If there
// are no digits, returns 0.
static unsigned char parseHex(const char *buffer) {
  unsigned char result = 0;
  while (isWhitespace(*buffer)) buffer++;
  while (*buffer) {
    unsigned char nibble = from_hex(*buffer++);
    if (nibble > 0x0f)
      break;
    result <<= 4;
    result |= nibble;
  }
  return result;
}

static void print(SerialCom *com, const char *str) {
  while (*str) com->write(*str++);
}

static void println(SerialCom *com, const char *str) {
  print(com, str); print(com, "\r\n");
}
static void printHex(SerialCom *com, unsigned char c) {
  com->write(to_hex(c >> 4));
  com->write(to_hex(c & 0x0f));
}

// A line buffer wrapping around the serial read. Nonblocking fills until either
// the buffer is full or newline reached.
// TODO: actually do clipping and not return the overlong line.
class LineBuffer {
public:
  LineBuffer() : pos_(buffer_) { }

  // Empties serial input buffer and stores in internal buffer.
  // Returns number of characters if newline reached or buffer full.
  // Returns '0' while this condition is not yet reached.
  byte ReadlineNoblock(SerialCom *comm) {
    const char *end_buf = buffer_ + sizeof(buffer_) - 1;
    bool newline_seen = false;
    while (!newline_seen && comm->read_available() && pos_ < end_buf) {
      const char c = (*pos_++ = comm->read());
      newline_seen = (c == '\r' || c == '\n');
    }
    *pos_ = '\0';  // We always have at least one byte space.
    if (newline_seen) *--pos_ = '\0';  // Don't return newline.
    if (newline_seen || pos_ >= end_buf) {
      byte len = pos_ - buffer_;
      pos_ = buffer_;
      return len;
    } else {
      return 0;
    }
  }

  // Returns current line, '\0' terminated, newline stripped.
  const char *line() const { return buffer_; }

private:
  char buffer_[32 + 1];
  char *pos_;
};

static void printHelp(SerialCom *out) {
  // Keep strings short or memory explodes :) The Harvard architecture of the
  // Atmel chips requires that the strings are first copied to memory.
  // Better split up in multiple calls with shorter strings.
  print(out,
        "? Noisebridge access terminal | v0.1 | 8/2014\r\n"
        "? Sends card-IDs with:\r\n"
        "?\tI<num-bytes-hex> <uid-hex-str>\r\n");
  print(out,"? Commands:\r\n"
        "?\t?\tThis help\r\n"
        "?\te<msg>\tEcho back msg (testing)\r\n"
        "?\tr\tReset reader.\r\n"
        "?\tM<n><msg> Write msg on LCD-line n=0,1.\r\n"
        "?\tW<xx>\tWrite output bits; param 8bit hex.\r\n");
  print(out, "? dropped-reads: 0x");
  printHex(out, out->dropped_reads() >> 8);
  printHex(out, out->dropped_reads() & 0xff);
  print(out, "\r\n");
}

static void setAuxBits(const char *buffer, SerialCom *out) {
  unsigned char value = parseHex(buffer + 1);
  value &= AUX_BITS;
  PORTC = value;
  out->write('W');
  printHex(out, value);
  println(out, "");
}

static void writeUid(const MFRC522::Uid &uid, SerialCom *out) {
  if (uid.size > 15) return;  // fishy.
  out->write('I');
  printHex(out, (unsigned char) uid.size);
  out->write(' ');
  for (int i = 0; i < uid.size; ++i) {
    printHex(out, uid.uidByte[i]);
  }
  println(out, "");
}

int main() {
  DDRC = AUX_BITS;

  _delay_ms(100);  // Wait for voltage to settle before we reset the 522

  MFRC522 card_reader;
  card_reader.PCD_Init();

  MFRC522::Uid current_uid;

  SerialCom comm;
  LcdDisplay lcd(24);
  lcd.print(0, "  Noisebridge  ");
  lcd.print(1, "");

  LineBuffer lineBuffer;
  println(&comm, "Noisebridge access control outpost. '?' for help.");
  int rate_limit = 0;

  for (;;) {
    // See if there is a command incoming.
    char line_len;
    if ((line_len = lineBuffer.ReadlineNoblock(&comm)) != 0) {
      switch (lineBuffer.line()[0]) {
      case '?':
        printHelp(&comm);
        break;
      case 'e':
        println(&comm, lineBuffer.line());
        break;
      case 'W':
        setAuxBits(lineBuffer.line(), &comm);
        break;
      case 'r':
        card_reader.PCD_Reset();
        card_reader.PCD_Init();
        current_uid.size = 0;
        println(&comm, "reset RFID reader.");
        break;
      case 'M':
        if (line_len >= 2 && lineBuffer.line()[1] - '0' < 2) {
          lcd.print(lineBuffer.line()[1] - '0', lineBuffer.line() + 2);
          println(&comm, "M ok");
        } else {
          println(&comm, "E row number must be 0 or 1");
        }
        break;
      case '\0': // TODO: the lineBuffer sometimes returns empty lines.
        break;
      default:
        print(&comm, "E Unknown command 0x");
        printHex(&comm, lineBuffer.line()[0]);
        println(&comm, "; '?' for help.");
      }
    }

    // While we still have bytes ready to read, handle these first, otherwise
    // we might run out of buffer because the RFID reading takes its sweet time.
    if (comm.read_available())
      continue;

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
