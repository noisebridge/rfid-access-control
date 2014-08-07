# <h.zeller@acm.org>
##

# CPU frequency and baudrate as used at compile time. We want to have
# a slow communication as we have pretty long lines and don't use RS422, so
# consider to reduce the baudrate for production use to something that works on
# the line.
DEFINES=-DF_CPU=8000000UL -DSERIAL_BAUDRATE=9600

TARGET_ARCH=-mmcu=atmega8
CXX=avr-g++
CXXFLAGS=-std=c++11 -Os -g -Wall -ffunction-sections -fdata-sections -mcall-prologues $(DEFINES)
AVRDUDE     = avrdude -p m8 -c stk500v2 -P /dev/ttyUSB0
FLASH_CMD   = $(AVRDUDE) -e -U flash:w:main.hex
LINK=avr-g++ -g $(TARGET_ARCH) -Wl,-gc-sections
OBJECTS=terminal-main.o mfrc522.o lcd.o serial-com.o keypad.o

all : main.hex

main.elf: $(OBJECTS)
	$(LINK) -o $@ $(OBJECTS)
	avr-size $@

# To compare different sized assembly outputs. Creates files with names including
# size
space-output: main.elf
	avr-objdump -S $^ > /tmp/terminal-`avr-size -d main.elf | grep main.elf | awk '{print $$4}'`.S

disasm: main.elf
	avr-objdump -S main.elf

main.hex: main.elf
	avr-objcopy -j .text -j .data -O ihex main.elf main.hex

read-eeprom:
	$(AVRDUDE) -U eeprom:r:-:i

flash: main.hex
	$(FLASH_CMD)

clean:
	rm -f $(OBJECTS) main.elf main.hex

# Documentation page references from
# http://www.atmel.com/images/atmel-2486-8-bit-avr-microcontroller-atmega8_l_datasheet.pdf / pp 216-217 are interesting.
# internal oscillator.
### Fuse high byte:
# 7 rstdisbl	1   disable external reset: disabled (i.e.: reset enabled).
# 6 wdton	1   watchdog.
# - spien	0   serial programming: enabled.
# 4 chkopt	1   clock options (doesn't matter, we do internal RC)
#
# 3 eesave      0   save eeprom on chip erase: enabled
# 2 bootsz1	1
# 1 bootsz0	1
# 0 bootrst     1   starting at 0x0000

### low byte:
# 7 bodlevel	1   brown out
# 6 boden	1   brown out
# 5 sut1	1-+ slow rising power (page 30)
# 4 sut0	0/
#
# 3 cksel3	0\
# 2 cksel2	1 + internal RC oscillator  (page 30)  (8Mhz)
# 1 cksel1	0/
# 0 cksel0	0
fuse:
	$(AVRDUDE) -U hfuse:w:0xd7:m -U lfuse:w:0xe4:m
