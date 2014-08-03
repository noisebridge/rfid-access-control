
#include <avr/eeprom.h>
#include <avr/io.h>
#include <string.h>
#include <util/delay.h>

#include "mfrc522.h"

#define EEPROM_SIZE 512
#define LED_PORT PORTC
#define READ_LED (1<<5)
#define ACCESS_LED (1<<4)
#define LED_BITS (READ_LED | ACCESS_LED)

enum Flags {
    FLAGS_NONE = 0x00,
    FLAGS_ADMIN = 0x01,
};

struct ShortUid {
    byte uidByte[4];
    byte flags;   // e.g. 'admin' etc.
};

#define MAX_UIDS (EEPROM_SIZE - 1) / sizeof(ShortUid)

struct IdStore {
    byte count;
    struct ShortUid uid[ MAX_UIDS ];
};

struct IdStore ee_store EEMEM;

static byte uid_count() {
    return eeprom_read_byte(&ee_store.count);
}


// Return 'true' if the uid 'to_find' can be found.
// Obliteterates 'found_buffer'. If the uid has been found, found_buf
// contains the given uid, otherwise is undefined.
static bool findUid(const MFRC522::Uid &to_find, ShortUid *found_buf) {
    if (to_find.size < sizeof(found_buf->uidByte))
        return false;
    const byte count = uid_count();
    for (byte i = 0; i < count; ++i) {
        eeprom_read_block(found_buf, &ee_store.uid[i], sizeof(*found_buf));
        if (memcmp(found_buf->uidByte, to_find.uidByte,
                   sizeof(found_buf->uidByte)) == 0)
            return true;
    }
    return false;
}

static bool storeUid(const MFRC522::Uid &to_store, byte flags) {
    const byte count = uid_count();
    if (count >= MAX_UIDS || to_store.size < sizeof(ee_store.uid[0].uidByte))
        return false;
    eeprom_write_block(to_store.uidByte, &ee_store.uid[count].uidByte,
                       sizeof(ee_store.uid[count].uidByte));
    eeprom_write_byte(&ee_store.uid[count].flags, flags);
    eeprom_write_byte(&ee_store.count, count + 1);
    return true;
}

int main() {
  DDRC = LED_BITS;

  MFRC522 card_reader;
  card_reader.PCD_Init();

  // Init 
  PORTC |= READ_LED;
  _delay_ms(100);
  PORTC &= ~READ_LED;

  PORTC |= ACCESS_LED;
  _delay_ms(100);
  PORTC &= ~ACCESS_LED;
  
  MFRC522::Uid current_uid;
  ShortUid access_uid;

  if (uid_count() == 0xff) {
      eeprom_write_byte(&ee_store.count, 0);
  }

  bool first_time = (uid_count() == 0);

  for (;;) {
    if (!card_reader.PICC_IsNewCardPresent())
      continue;
    if (!card_reader.PICC_ReadCardSerial())
      continue;
    current_uid = card_reader.uid;

    PORTC |= READ_LED;

    if (first_time) {
        // The first time, blank memory, the first key we see is the master key
        storeUid(current_uid, FLAGS_ADMIN);
        first_time = false;
    }

    if (findUid(current_uid, &access_uid)) {
        PORTC |= ACCESS_LED;
        _delay_ms(100);
        PORTC &= ~ACCESS_LED;
    }

    while (card_reader.PICC_ReadCardSerial()
           && current_uid.size == card_reader.uid.size
           && memcmp(current_uid.uidByte,
                     card_reader.uid.uidByte, card_reader.uid.size) == 0) {
    }
    PORTC &= ~READ_LED;
  }
}
