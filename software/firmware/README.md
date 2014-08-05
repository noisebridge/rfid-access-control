Firmware to run on the external nodes, interfacing a RFID-RC522 type reader
providing a simple serial interface to be read by the host.

Based on Atmega8 (because I had a bunch lying around).

Uses https://github.com/miguelbalboa/rfid to interface with the RC522 board,
but hacked to not depend on Arduino libraries.
It only uses a tiny subset of that library: to read the UID (which could
probably be simplified, because it is quite a chunk of code). But hey, it was
already there and I have 8k to waste.

The serial protocol communicates with 9600 8N1 (TODO: reconsider speed if we go
with RS232 instad of RS422 on the physical wire). Whenever a new card is seen,
a line is sent to the host

     Rxx yyyyyyyy

With xx being the number of bytes (RFID IDs come in 4, 7 and 10 bytes) followed
by the actual bytes. All numbers (xx and yy...) are in hex.

Also responds to one-line commands from the host (all commands must be
sent with `<CR>` or `<LF>` or both). The first character in each line denotes
the command, followed by optional parameters.

     ?     : Prints help.
     S<xx> : set output pins. Understands 8-bit hex number, but only
             6 bits are currently addressed: PC[0..5] on the Atmega8
             Responds with S<yy> with <yy> the actual bits being set.
             Can be used to switch on fancy LEDs or even remotely trigger a
             relay or transistor to open the strike.
             Example:

             S ff

     M<r><msg> : Write a message to the LCD screen. Example:

		 M1Hello World

                 writes this message to the second line.
     r     : Reset RFID reader (Should typically not be necessary except after
             physical connection reconnect of its SPI bus).
     P     : Ping; responds with "Pong". Useful to check aliveness.

Responses generally are prefixed with the letter of the command. Makes
interfacing simple.

To compile, you need the avr toolchain installed:

     sudo aptitude install gcc-avr avr-libc avrdude

Work in progress :)

![Work in progress][work]

[work]: https://github.com/hzeller/rfid-access-control/raw/master/img/work-in-progress.jpg
