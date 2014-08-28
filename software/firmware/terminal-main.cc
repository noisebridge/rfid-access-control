/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 * TODO: replace all counts of [unsigned] char, [unsigned] short with the
 * appropriate [u]int{8,16}_t. Only strings should deal with 'char'.
 */
#include <avr/eeprom.h>
#include <avr/io.h>
#include <string.h>
#include <util/delay.h>
#include <avr/pgmspace.h>

#include "clock.h"
#include "keypad.h"
#include "lcd.h"
#include "mfrc522.h"
#include "serial-com.h"

#define AUX_PORT PORTC
#define AUX_BITS 0x3F

// Pointer to progmem string. Wrapped into separate type to have a type-safe
// way to deal with it.
struct ProgmemPtr {
  explicit ProgmemPtr(const char *d) : data(d) {}
  const char *data;
};
#define _P(s) ProgmemPtr(PSTR((s)))

// TODO: move this repository to noisebridge github.
const char kCodeUrl[] PROGMEM = "https://github.com/hzeller/rfid-access-control";
const char kHeaderText[] PROGMEM = "Noisebridge access terminal | v0.1 | 8/2014";

// Don't change sequence in here. Add stuff at end. This is the
// raw layout in our eeprom which shouldn't change :)
struct EepromLayout {
  char name[32];      // Shall be nul terminated. So at most 31 long.
  uint16_t baud_rate; // If garbage, falls back to SERIAL_BAUDRATE
  // other things here.
};
// EEPROM layout with some defaults in case we'd want to prepare eeprom flash.
struct EepromLayout EEMEM ee_data = {
  /* .name      = */ "terminal",
  /* .baud_rate = */ SERIAL_BAUDRATE,
};

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

#if ALLOW_BAUD_CHANGE
// Like parseHex(), but decimal numbers.
static uint16_t parseDec(const char *buffer) {
  uint16_t result = 0;
  while (isWhitespace(*buffer)) buffer++;
  while (*buffer) {
    unsigned char nibble = from_hex(*buffer++);
    if (nibble > 10)
      break;
    result *= 10;
    result += nibble;
  }
  return result;
}
#endif

#if ALLOW_BAUD_CHANGE
static uint16_t GetBaudEEPROM() {
  return eeprom_read_word(&ee_data.baud_rate);
}
static void StoreBaudEEPROM(uint16_t bd) {
  return eeprom_write_word(&ee_data.baud_rate, bd);
}
#endif

// Requires buffer with >= 32 bytes space (sizeof(ee_data.name)).
static const char *GetNameEEPROM(char *buffer) {
  eeprom_read_block(buffer, &ee_data.name, sizeof(ee_data.name));
  buffer[sizeof(ee_data.name) - 1] = '\0';  // Ensure termination.
  return buffer;
}

// Store NUL terminated string in eeprom. But not more than 31+1 bytes.
static void StoreNameEEPROM(const char *name) {
  uint8_t len = strlen(name);
  if (len >= sizeof(ee_data.name)) {
    len = sizeof(ee_data.name) - 1;
  }
  eeprom_write_block(name, &ee_data.name, len);
  for (int i = len; i < (int)sizeof(ee_data.name); ++i) { // Rest NUL bytes.
    eeprom_write_byte((uint8_t*)&ee_data.name + i, '\0');
  }
}

// Some convenience methods around the serial line.
static void print(SerialCom *out, ProgmemPtr str) {
 char c;
 while ((c = pgm_read_byte(str.data++)) != 0x00)
   out->write(c);
}
static void println(SerialCom *out) {
  print(out, _P("\r\n"));
}
static void println(SerialCom *out, ProgmemPtr str) {
  print(out, str);
  println(out);
}
// Unlike the typically used progmem versions, print buffer from RAM. Named
// obnoxiously so that typically the ProgmemPtr versions are considered.
static void printlnFromRam(SerialCom *out, const char *str) {
  while (*str) out->write(*str++);
  println(out);
}
static void printHexByte(SerialCom *out, unsigned char c) {
  out->write(to_hex(c >> 4));
  out->write(to_hex(c & 0x0f));
}
static void printHexShort(SerialCom *out, unsigned short s) {
  printHexByte(out, (s >> 8));
  printHexByte(out, (s & 0xff));
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

static void SendHelp(SerialCom *out) {
  print(out, _P("? "));
  println(out, ProgmemPtr(kHeaderText));
  print(out, _P("# "));
  println(out, ProgmemPtr(kCodeUrl));

  print(out,
        _P("# [Sends]\r\n"
           "#\tI<num-bytes-hex> <uid-hex-str> RFID in range.\r\n"
           "#\tK<char>\tPressed keypad char 0..9, '*','#'\r\n"
           "#\r\n"
           "# [Commands]\r\n"
           "# Lower case: read state\r\n"
           "#\t?\tThis help\r\n"
           "#\tn\tGet persistent name.\r\n"
           "#\ts\tShow stats.\r\n"
           "#\te<msg>\tEcho back msg (testing)\r\n"
           "#\r\n"
           "# Upper case: modify state\r\n"
           "#\tM<n><msg> Write msg on LCD-line n=0,1.\r\n"
           "#\tW<xx>\tWrite output bits; param 8bit hex.\r\n"
           "#\tR\tReset RFID reader.\r\n"
           "#\tN<name> Set persistent name of this terminal. Send twice.\r\n"
#if ALLOW_BAUD_CHANGE
           "#\tB<baud> Set baud rate. Persists if current rate confirmed.\r\n"
#endif
           ));
}

static void SendStats(SerialCom *out, unsigned short cmd_count) {
  print(out, _P("s commands-seen=0x"));
  printHexShort(out, cmd_count);
  print(out, _P("; dropped-rx-bytes=0x"));
  printHexShort(out, out->dropped_rx());
  print(out, _P("\r\n"));
}

static void SetAuxBits(const char *buffer, SerialCom *out) {
  unsigned char value = parseHex(buffer + 1);
  value &= AUX_BITS;
  PORTC = value;
  out->write('W');
  printHexByte(out, value);
  println(out);
}

// We require to send the same name twice in consecutive commands to make
// sure to not set the name due to accidents or random line noise.
static uint8_t first_name_write_command_count = 0x42;
static uint8_t name_checksum;
static void ReceiveName(SerialCom *com,
                        const char *line, uint8_t command_count) {
  uint8_t checksum = 0;
  const char *cs_run = line;
  for (uint8_t i = 0; *cs_run; ++i, ++cs_run)
    checksum ^= *cs_run + i;  // crude, but should catch typos.
  if (first_name_write_command_count + 1 == command_count) {
    // The previous command was name setting as well. See if we got the same.
    if (checksum == name_checksum) {
      StoreNameEEPROM(line + 1);
      print(com, _P("Name set: "));
      printlnFromRam(com, line + 1);
    } else {
      println(com, _P("Name mismatch!"));
    }
  } else {
    first_name_write_command_count = command_count;
    name_checksum = checksum;
    println(com, _P("Name received. Send 2nd time to confirm."));
  }
}

#if ALLOW_BAUD_CHANGE
static void SetNewBaudRate(SerialCom *com, const char *line) {
  const uint16_t bd = parseDec(line + 1);
  if (!SerialCom::IsValidBaud(bd)) {
    println(com, _P("E not a valid baudrate between 300..38400"));
    return;
  }
  if (bd == com->baud()) {
    // We are already running at that speed. Obviously communication works.
    // Safe to store permanently.
    StoreBaudEEPROM(bd);
    println(com, _P("Baud rate stored in EEPROM"));
  } else {
    println(com, _P("Baud rate will be switched after this line. Send command "
                    "a second time to permanently store in EEPROM"));
    com->SetBaud(bd);
  }
}
#endif

static void SendUid(const MFRC522::Uid &uid, SerialCom *out) {
  if (uid.size > 15) return;  // fishy.
  out->write('I');
  printHexByte(out, (unsigned char) uid.size);
  out->write(' ');
  for (int i = 0; i < uid.size; ++i) {
    printHexByte(out, uid.uidByte[i]);
  }
  println(out);
}

static void SendKeypadCharIfAvailable(SerialCom *out, char keypad_char) {
  if (!keypad_char) return;
  out->write('K');
  out->write(keypad_char);
  println(out);
}

int main() {
  DDRC = AUX_BITS;

  _delay_ms(100);  // Wait for voltage to settle before we reset the 522

  char buffer[32];  // general purpose. Allocated once for no stack-surprises.

  Clock::init();

  KeyPad keypad;
  LcdDisplay lcd(24);

  MFRC522 card_reader;
  card_reader.PCD_Init();

  SerialCom comm;
#if ALLOW_BAUD_CHANGE
  comm.SetBaud(GetBaudEEPROM());
#endif

  print(&comm, _P("# "));
  print(&comm, ProgmemPtr(kHeaderText));
  println(&comm, _P("; '?' for help."));
  print(&comm, _P("# "));
  println(&comm, ProgmemPtr(kCodeUrl));
  print(&comm, _P("# Name: "));
  printlnFromRam(&comm, GetNameEEPROM(buffer));

  LineBuffer lineBuffer;
  MFRC522::Uid current_uid;

  int rate_limit = 0;
  unsigned short commands_seen_stat = 0;
  for (;;) {
    // See if there is a command incoming.
    char line_len;
    if ((line_len = lineBuffer.ReadlineNoblock(&comm)) != 0) {
      ++commands_seen_stat;
      switch (lineBuffer.line()[0]) {
      case '?':
        SendHelp(&comm);
        break;
        // Commands that modify stuff. Upper case letters.
      case 'W':
        SetAuxBits(lineBuffer.line(), &comm);
        break;
      case 'R':
        card_reader.PCD_Reset();
        card_reader.PCD_Init();
        current_uid.size = 0;
        println(&comm, _P("Reset RFID reader."));
        break;
      case 'M':
        if (line_len >= 2 && lineBuffer.line()[1] - '0' < 2) {
          lcd.print(lineBuffer.line()[1] - '0', lineBuffer.line() + 2);
          println(&comm, _P("M ok"));
        } else {
          println(&comm, _P("E row number must be 0 or 1"));
        }
        break;
      case 'N':
        ReceiveName(&comm, lineBuffer.line(), commands_seen_stat & 0xff);
        break;
#if ALLOW_BAUD_CHANGE
      case 'B':
        SetNewBaudRate(&comm, lineBuffer.line());
        break;
#endif
        // Lower case letters don't modify any state.
      case 'e':
        printlnFromRam(&comm, lineBuffer.line());
        break;
      case 's':
        SendStats(&comm, commands_seen_stat);
        break;
      case 'n':
        comm.write('n');
        printlnFromRam(&comm, GetNameEEPROM(buffer));
        break;
      case '\0': // TODO: the lineBuffer sometimes returns empty lines.
        break;
      default:
        print(&comm, _P("E Unknown command 0x"));
        printHexByte(&comm, lineBuffer.line()[0]);
        println(&comm, _P("; '?' for help."));
      }
    }

    // While we still have bytes ready to read, handle these first, otherwise
    // we might run out of buffer because the RFID reading takes its sweet time.
    if (comm.read_available())
      continue;

    SendKeypadCharIfAvailable(&comm, keypad.ReadKeypad());

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
    SendUid(current_uid, &comm);
  }
}
