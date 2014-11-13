/* -*- mode: c++; c-basic-offset: 2; indent-tabs-mode: nil; -*-
 * Copyright (c) h.zeller@acm.org. GNU public License.
 * TODO: replace all counts of [unsigned] char, [unsigned] short with the
 * appropriate [u]int{8,16}_t. Only strings should deal with 'char'.
 */
#include <avr/eeprom.h>
#include <avr/io.h>
#include <avr/pgmspace.h>
#include <string.h>
#include <util/delay.h>

#include "clock.h"
#include "keypad.h"
#include "lcd.h"
#include "mfrc522/mfrc522.h"
#include "serial-com.h"
#include "tone-gen.h"

#if FEATURE_RFID_DEBUG
#  include "mfrc522-debug.h"
#endif

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
const char kHeaderText[] PROGMEM = "Noisebridge access terminal | v0.2 | 8/2014";

// TODO: make configurable. This represents the layout downstairs.
enum { RED_LED   = 0x20,   // LCD-EN
       GREEN_LED = 0x10,   // LCD-RS
       BLUE_LED  = 0x02 };  // LCD-D1

// Don't change sequence in here. Add stuff at end. This is the
// raw layout in our eeprom which shouldn't change :)
// We store flags in full bytes for convenience reasons.
struct EepromLayout {
  char name[32];      // Shall be nul terminated. So at most 31 long.
  uint16_t baud_rate; // If garbage, falls back to SERIAL_BAUDRATE
  uint8_t flag_keyboard_tone;  // Use a keyboard tone.
  // other things here.
};

// EEPROM layout with some defaults in case we'd want to prepare eeprom flash.
struct EepromLayout EEMEM ee_data = {
  /* .name      = */          "terminal",
  /* .baud_rate = */          SERIAL_BAUDRATE,
  /* .flag_keyboard_tone = */ 1,
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

inline static bool GetFlag(uint8_t* which) { return eeprom_read_byte(which); }
inline static bool SetFlag(uint8_t* which, bool value) {
  eeprom_write_byte(which, value);
  return value;
}

#if FEATURE_BAUD_CHANGE
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
  // If the eeprom just contains the default state (all 0xFF, which is not
  // a useful ASCII character to begin with), just return an empty string.
  if (buffer[0] == 0xff)
    buffer[0] = '\0';
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
static void printlnFromRAMPointer(SerialCom *out, const char *str) {
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

static void PrintShortHeader(SerialCom *out) {
  print(out, _P("# "));
  println(out, ProgmemPtr(kHeaderText));
  print(out, _P("# "));
  println(out, ProgmemPtr(kCodeUrl));
}

static void SendHelp(SerialCom *out) {
  PrintShortHeader(out);
  print(out,
        _P("# [Sends]\r\n"
           "#\tI<num-bytes-hex> <uid-hex-str> RFID in range.\r\n"
           "#\tK<char>\tPressed keypad char 0..9, '*','#'\r\n"
           "#\r\n"
           "# [Commands]\r\n"
           "# Lower case: read state\r\n"
           "#\t?\tThis help\r\n"
           "#\tn\tGet persistent name.\r\n"
#if FEATURE_RFID_DEBUG
           "#\tr\tShow MFRC522 registers.\r\n"
#endif
           "#\ts\tShow stats.\r\n"
           "#\te<msg>\tEcho back msg (testing)\r\n"
           "#\r\n"
           "# Upper case: modify state\r\n"
#if FEATURE_LCD
           // We either support the LCD or the LED on that port
           "#\tM<n><msg> Write msg on LCD-line n=0,1.\r\n"
#else
           "#\tL[<R|G|B>] Set (combination of) LED Red/Green/Blue.\r\n"
#endif
           "#\tT<L|H>[<ms>] Low or High tone for given time (default 250ms).\r\n"
           "#\tF<K><1|0> Set flag. 'K'=Keypad click.\r\n"
           "#\tR\tReset RFID reader.\r\n"
           "#\tN<name> Set persistent name of this terminal. Send twice.\r\n"
#if FEATURE_BAUD_CHANGE
           "#\tB<baud> Set baud rate. Persists if current rate confirmed.\r\n"
#endif
           ));
  println(out, _P("? ok"));
}

static void SendStats(SerialCom *out, unsigned short cmd_count) {
  print(out, _P("s commands-seen=0x"));
  printHexShort(out, cmd_count);
  print(out, _P("; dropped-rx-bytes=0x"));
  printHexShort(out, out->dropped_rx());
  print(out, _P("\r\n"));
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
      printlnFromRAMPointer(com, line + 1);
    } else {
      println(com, _P("Name mismatch!"));
    }
  } else {
    first_name_write_command_count = command_count;
    name_checksum = checksum;
    println(com, _P("Name received. Send 2nd time to confirm."));
  }
}

static void OutputTone(SerialCom *com, const char *line) {
  uint16_t duration = parseDec(line + 2);
  if (duration == 0) duration = 250;
  if (line[1] == 'H' || line[1] == 'h') {
    ToneGen::Tone(ToneGen::hz_to_divider(1200), Clock::ms_to_cycles(duration));
  } else {
    ToneGen::Tone(ToneGen::hz_to_divider(300), Clock::ms_to_cycles(duration));
  }
  println(com, _P("T ok"));
}

#if not FEATURE_LCD
static void ResetLED() {
  PORTC |= RED_LED|GREEN_LED|BLUE_LED;
}
static void SetLED(SerialCom *com, const char *line) {
  ResetLED();
  for (const char *color = line+1; *color ; color++) {
    switch (*color) {
    case 'R': case 'r': PORTC &= ~RED_LED; break;
    case 'G': case 'g': PORTC &= ~GREEN_LED; break;
    case 'B': case 'b': PORTC &= ~BLUE_LED; break;
    }
  }
  println(com, _P("L ok"));
}
#endif

static void SetFlagCommand(SerialCom *com, const char *line) {
  bool result;
  switch (line[1]) {
  case 'K':
    result = SetFlag(&ee_data.flag_keyboard_tone, line[2] == '1');
    break;
  default:
    println(com, _P("E invalid flag"));
    return;
  }
  println(com, result ? _P("T flag on") : _P("T flag off"));
}

#if FEATURE_BAUD_CHANGE
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

static void SendKeypadCharIfAvailable(char keypad_char, SerialCom *out) {
  if (!keypad_char) return;
  out->write('K');
  out->write(keypad_char);
  println(out);
  if (GetFlag(&ee_data.flag_keyboard_tone)) {
    ToneGen::Tone(ToneGen::hz_to_divider(1000), Clock::ms_to_cycles(30));
  }
}

int main() {
  DDRC = AUX_BITS;

#if not FEATURE_LCD
  ResetLED();
#endif

  _delay_ms(100);  // Wait for voltage to settle before we reset the 522

  char buffer[32];  // general purpose. Allocated once for no stack-surprises.

  Clock::init();

  ToneGen::Init();
  KeyPad keypad;

#if FEATURE_LCD
  LcdDisplay lcd(24);
#endif

  MFRC522 card_reader;
  card_reader.PCD_Init();

  SerialCom comm;
#if FEATURE_BAUD_CHANGE
  comm.SetBaud(GetBaudEEPROM());
#endif

  PrintShortHeader(&comm);
  println(&comm, _P("# Type '?<RETURN>' for help."));
  print(&comm, _P("# Name: "));
  printlnFromRAMPointer(&comm, GetNameEEPROM(buffer));

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
      case 'R':
        card_reader.PCD_Reset();
        card_reader.PCD_Init();
        current_uid.size = 0;
        println(&comm, _P("Reset RFID reader."));
        break;
#if FEATURE_LCD
      case 'M':
        if (line_len >= 2 && lineBuffer.line()[1] - '0' < 2) {
          lcd.print(lineBuffer.line()[1] - '0', lineBuffer.line() + 2);
          println(&comm, _P("M ok"));
        } else {
          println(&comm, _P("E row number must be 0 or 1"));
        }
        break;
#else
      case 'L':
        SetLED(&comm, lineBuffer.line());
        break;
#endif
      case 'N':
        ReceiveName(&comm, lineBuffer.line(), commands_seen_stat & 0xff);
        break;
#if FEATURE_BAUD_CHANGE
      case 'B':
        SetNewBaudRate(&comm, lineBuffer.line());
        break;
#endif
      case 'T':
        OutputTone(&comm, lineBuffer.line());
        break;
      case 'F':
        SetFlagCommand(&comm, lineBuffer.line());
        break;
        // Lower case letters don't modify any state.
      case 'e':
        printlnFromRAMPointer(&comm, lineBuffer.line());
        break;
#if FEATURE_RFID_DEBUG
      case 'r':
        showRFIDStatus(&comm, &card_reader);
        break;
#endif
      case 's':
        SendStats(&comm, commands_seen_stat);
        break;
      case 'n':
        comm.write('n');
        printlnFromRAMPointer(&comm, GetNameEEPROM(buffer));
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

    SendKeypadCharIfAvailable(keypad.ReadKeypad(), &comm);

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
