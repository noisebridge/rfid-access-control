
#include <avr/io.h>
#include <string.h>
#include <util/delay.h>

#include "mfrc522.h"

#define LED_PORT PORTC
#define LED_BITS (1<<5)

int main() {
  DDRC = LED_BITS;

  MFRC522 card_reader;
  card_reader.PCD_Init();

  // Init 
  PORTC |= LED_BITS;
  _delay_ms(1000);
  PORTC &= ~LED_BITS;
  
  MFRC522::Uid current_uid;

  for (;;) {
    if (!card_reader.PICC_IsNewCardPresent())
      continue;
    if (!card_reader.PICC_ReadCardSerial())
      continue;
    current_uid = card_reader.uid;
    PORTC |= LED_BITS;
    while (card_reader.PICC_ReadCardSerial()
           && current_uid.size == card_reader.uid.size
           && memcmp(current_uid.uidByte,
                     card_reader.uid.uidByte, card_reader.uid.size) == 0) {
    }
    PORTC &= ~LED_BITS;
  }
}
