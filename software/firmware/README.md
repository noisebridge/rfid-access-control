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

We only use RX and TX, so hardware flow control needs to be switched off. Other
communication parameters are 9600 8N1 (TODO: reconsider speed if we go
with RS232 instad of RS422 on the physical wire).

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

     ?     : Prints help.
     W<xx> : Writes output pins. Understands 8-bit hex number, but only
             6 bits are currently addressed: PC[0..5] on the Atmega8
             Responds with W<yy> with <yy> the actual bits being set (some
             might not be available).
             Can be used to switch on fancy LEDs or even remotely trigger a
             relay or transistor to open the strike.
             Example:

             W ff

     M<r><msg>
           : Write a message to the LCD screen. <r> is a single digit
             giving the row to print in, can be 0 or 1).
             Example:

                 M1Hello World

             writes this message to the second line.
     R     : Reset RFID reader (Should typically not be necessary except after
             physical connection reconnect of its SPI bus).
     e<msg>: Just echo back given message. Useful for line-reliability test.
             (Use with line length ~ <= 30 characters)
     s     : Read stats.
     (TODO: specialized command to buzz or silent open, color leds etc)

Each command is acknowledged with a line prefixed with the letter of the
command, *or* on error, the returned value starts with `E`.
Makes interfacing the protocol simple.

Compiling
---------
To compile, you need the avr toolchain installed:

     sudo aptitude install gcc-avr avr-libc avrdude

... and a programmer.

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

(Locally, it works fine up to 38400, but with long lines I'd measure the
best speed that doesn't produce any errors and then use half that).

Hacking in progress:

![Work in progress][work]

[work]: https://github.com/hzeller/rfid-access-control/raw/master/img/work-in-progress.jpg
[rfid-board]: https://github.com/hzeller/rfid-access-control/raw/master/img/rfid-rc522.jpg
[lcd]: https://github.com/hzeller/rfid-access-control/raw/master/img/lcd-connector.jpg