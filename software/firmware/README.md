Access Terminal Firmware
========================

Firmware to run on the external nodes, typically featuring an RFID-RC522 reader
and potentially other peripheral such as a Keypad or an LCD screen. Some
may have an H-Bridge to switch or buzz a door strike.

They interface with a serial interface with a simple protocol with the host.

Based on Atmega8 (because I had a bunch lying around). Running on 3.3V as
this is the voltage needed by the RFID reader.

Peripherals
-----------

### RFID-RC522

The reader to interface is the RFID-RC522; there are a bunch available everywhere
and they are cheap - in the order of $5. They interface via SPI with the
microcontroller. Uses https://github.com/miguelbalboa/rfid to interface with
the RC522 board, but hacked to not depend on Arduino libraries.

It only uses a tiny subset of that library: to read the UID (which could
probably be simplified, because it is quite a chunk of code). But hey, it was
already there and I have 8kiB to waste (also, I tell the linker to throw out
unused stuff as much as possible).

Uses the SPI port and part of port B.

This is how a typical board looks like

![RFID-board][rfid-board]

Connections from board to Atmega8
   - **SDA** _(-CS)_ to **PB2**   (Pin 16 on PDIP Atmega8)
       (and good to have a pull-up to 3.3V so that in-circuit programming works
        reliably)
   - **SCK** to **SCK**   (Pin 19 on PDIP Atmega8)
   - **MOSI** to **MOSI** (Pin 17 on PDIP Atmega8)
   - **MISO** to **MISO** (Pin 18 on PDIP Atmega8)
   - **IRQ** (not connected)
   - **GND** to **GND**
   - **RST** _(-Reset)_ to **PB1**   (Pin 15 on PDIP Atmega8)
   - **3.3V** to **3.3V**

### LCD

If this terminal should have an LCD to display messages or interact wit the
user, then these are connected to PC0..PC5 to the microcontroller.
(TODO: make it #define-able which features are available)

Unfortunately, many displays only work with 5V instead of 3.3V. Might need
dual voltage on the board just to power the display.

![LCD connector][lcd]

The LCD typically has 14 or 16 connector pins. Connections from LCD to Atmega8
   - **LCD 1** _(GND)_ to **GND**
   - **LCD 2** _(+5V)_ to **5V**
   - **LCD 3** _(contrast)_ to **GND**
       _the contrast is controllable with a resistor, but connecting it to GND
       is just fine_
   - **LCD 4** _(RS)_ to **PC4** (Pin 27 on PDIP Atmega8)
   - **LCD 5** _(R/-W)_ to *GND** _We only write to the display,
      so we set this pin to GND_
   - **LCD 6** _(Clock or Enable)_ to **PC5** (Pin 28 on PDIP Atmega8)
   - LCD 7, 8, 9, 10 are _not connected_
   - **LCD 11** _(Data 4)_ to **PC0** (Pin 23 on PDIP Atmega8)
   - **LCD 12** _(Data 5)_ to **PC1** (Pin 24 on PDIP Atmega8)
   - **LCD 13** _(Data 6)_ to **PC2** (Pin 25 on PDIP Atmega8)
   - **LCD 14** _(Data 7)_ to **PC3** (Pin 26 on PDIP Atmega8)
   - If availble: LCD 15 and LCD 16 are for background light.

### Keypad

Can read a standard 3 column x 4 row phone style keypad and output its debounced
values on the serial line.

The keypards typically come with four 'row' connectors and three 'column'
connectors, 7 pins total.
   - **Row 0** to **PD4** (Pin 6 on PDIP Atmega8)
   - **Row 1** to **PD5** (Pin 11 on PDIP Atmega8)
   - **Row 2** to **PD6** (Pin 12 on PDIP Atmega8)
   - **Row 3** to **PD7** (Pin 13 on PDIP Atmega8)
   - **Col 0** to **PB0** (Pin 14 on PDIP Atmega8)
   - **Col 1** to **PB6** (Pin 9 on PDIP Atmega8)
   - **Col 2** to **PB7** (Pin 10 on PDIP Atmega8)

### Output Pins

Output bits in PC0..PC5 if there is no LCD display. (TBD: and some leftover pins)

Serial Protocol
---------------
Simple line based protocol (the previous generation of the Noisebridge
access terminal was character based, but this doesn't work well anymore with
increased functionality).

Each value **sent** from the terminal comes as full line
(or lines in case of the help command). All lines end with `<CR><LF>`
because that displays well out of the box with any terminal emulator
(e.g. minicom), without too much configuration.
Lines **received** from the host are accepted with `<CR>` or `<LF>` or both.
Lines received are clipped if too long.

We only use RX and TX, so hardware flow control needs to be switched off. Default
communication parameters are **9600 8N1**, but you can set the baud-rate
later and store in EEPROM or compile in a different speed.

### Terminal -> Host

#### RFID

Whenever a new RFID card is seen, its ID is sent to the host:

     Ixx yyyyyyyy<CR><LF>

(Example: `I07 c41abefa24238d`)
With xx being the number of bytes (RFID IDs come in 4, 7 and 10 bytes). All
numbers are in hex, so values for length would be one of `04`, `07`, `0a`,
followed by the actual bytes as hex-string.

While the card is in range, this line is repeated every couple of 100ms.

#### Keypad
Each Key pressed on the phonepad is transmitted with a single line

     K*<CR><LF>

The star representing the key in this case.

### Host -> Terminal

The terminal also responds to one-line commands from the host.
They are one-letter commands, followed by parameters and end
with a `<CR>` or `<LF>` or both. Commands with upper-case letters modify
the state of the system, lower-case letters just read information.

Type `?` (+ newline) to get help over the serial line.

     # The following, lower-case letters read state, don't modify
     ?       : Prints help.
     n       : Read name of terminal as set with 'N'.
     s       : Read stats.
     e<msg>  : Just echo back given message. Useful for line-reliability test.
               (Use with line length ~ <= 30 characters).

     # Commands with upper-case letters modify the state.
     M<r><msg>
             : Write a message to the LCD screen. <r> is a single digit
               giving the row to print in, can be 0 or 1).
               The following example writes 'Hello World' into the second line:

                 M1Hello World

     W<xx>   : Writes output pins. Understands 8-bit hex number, but only
               6 bits are currently addressed: PC[0..5] on the Atmega8
               Responds with W<yy> with <yy> the actual bits being set (some
               might not be available).
               Can be used to switch on fancy LEDs or even remotely trigger a
               relay or transistor to open the strike.
               The following example sets all the bits:

               W ff

     R       : Reset RFID reader (Mostly debug; should typically not be necessary
               except after physical connection reconnect of its SPI bus).
     N<name> : Persistently set the name of this terminal. To avoid
               accidentally setting this, it prompts you to be called twice.
     B<baud> : Set baudrate. Accepts one of the common baudrate
               values { 300, 600, 1200, 2400, 4800, 9600, 19200, 38400 }.
               This changes the baudrate when this command returns.
               If it is changed, it is _not_ stored in EEPROM, so that you
               can test the new setting with changed parameters in your terminal
               program. If you can't communicate anymore, a power-cycle brings
               the terminal back to the previous baud-rate.
               If you are already at the baud-rate you specify (which obviously
               means that you _can_ communicate), it is persisted in EEPROM:
               Next time the terminal comes up, it will use this baudrate.
               So essentially: similar to 'N', you have to give this command
               twice to actually persist a new baudrate.

     (TODO: specialized command to buzz or silent open, color leds etc)

Each command is acknowledged with exactly one line prefixed with the letter of
the command, *or* on error in that command, the returned line starts with `E`.
Lines starting with `#` are informal for user interaction (e.g. startup screen
or help output) and should just be skipped by programmatic interfaces.
Makes interfacing the protocol simple.

Compiling
---------
To compile, you need the avr toolchain installed:

     sudo aptitude install gcc-avr avr-libc avrdude

... and a programmer.

    make fuse   # will set the right fuses in the programmer (first time)
    make flash  # compiles code and flashes it to the system.

If your avrdude is not connected to `/dev/ttyUSB0`, you can set the actual
device via environment variable:

    AVRDUDE_DEVICE=/dev/ttyUSB1 make flash

See Makefile for details.

Testing line speed
------------------
To test if the line speed is working for the given environment (long cable,
noisy line etc.) you can use the `e`-cho command to see if you get the same
data out as you get in. Let's generate some test-data and compare input and
output.

     $ i=1 ; while [ $i -lt 5000 ] ; do echo -en "eTest Echo $i\r\n" ; i=$[i+1]; done > /tmp/a.txt
     $ cat /tmp/a.txt | socat -b 256 STDIO /dev/ttyUSB1,raw,echo=0,b9600 > /tmp/b.txt
     $ diff /tmp/a.txt /tmp/b.txt

NB: Make sure to send each line with `<CR><LF>` (\r\n), as it is echoed with this
line-ending, indepenent of the input line-ending.
If you only send one character (e.g. \n), then the echo command would have to
write more data out than comes in - which would run out of TX buffer on writing.

While experimenting, just use the `B<baud>` command to change to different
speeds (With a short cable, it works fine up to 38400, but with long lines
I'd measure the best speed that doesn't produce any errors and then use
half that). (TODO: report back what actually worked for the 10+meter cable
at Noisebridge)

Setting up a new Terminal
-------------------------
A checklist:

   - Choose default baudrate and compile-time defines in Makefile.

   - First time set-up of Atmega8: `make fuse`

   - Writing EEPROM settings (default name and baudrate): `make eeprom-flash`
     (you typically want to do that only once, later these settings can be
     changed via the protocol).

   - General compiling and flashing: `make flash`

   - Connect with a terminal program (e.g. `minicom`), test
     connected LEDs, buzzer, LCD etc. using the terminal interface and that
     RFID reader and/or keypad return data.

   - Optional: Test line speed as described above to optimize for your setup.

     Note, inputs from RFID or keypad are fine with lower speeds
     (300 or 600 baud), thus more resilient to long cables; if you have an
     LCD connected, you might want more for more 'snappy' user-interaction.

   - Choose a name for the terminal to be used in your system. That way,
     the host-software can distinguish the terminals no matter what serial
     line they are connected to. Use descriptive names, e.g. _'Gate Downstairs'_
     or _'Elevator-3rd-floor'_. The command to use is `N<name>`, see serial
     protocol description; you need to set it _twice_ to store permanently.
     Check the current name with the lowercase `n` command.

   - Optionally check the contents of the eeprom if you are
     curious: `make read-eeprom`

     The first 32 bytes contain the name, followed by the baudrate.

Hacking in progress:

![Work in progress][work]

[work]: https://github.com/hzeller/rfid-access-control/raw/master/img/work-in-progress.jpg
[rfid-board]: https://github.com/hzeller/rfid-access-control/raw/master/img/rfid-rc522.jpg
[lcd]: https://github.com/hzeller/rfid-access-control/raw/master/img/lcd-connector.jpg
