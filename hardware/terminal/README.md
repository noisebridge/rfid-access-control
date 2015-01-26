Terminal PCB
============

The Access terminal mounted at the outside of the door. Typically connecting the
RFID reader (via SPI interface), but also a 4x3 keypad (or just simple door-bell
button) and an LCD can be connected. If wanted, even an electric strike via
some external H-bridge.

This is based on an Atmega8 because I had a bunch lying around, but the DIL
version is bulky and ideally, I'd like to have a bit more IO pins, such as
for a 4th row of the keypad or separate LED outputs; future versions might
change to atmega168 (currently the best Atmel-IO bang for the buck).

This board provides the necessary breakouts to connect the peripherals to an
Atmega8 and holds the MAX232 compatible line driver.
The software in `../../software/firmware` makes it useful.

For a housing of this board and the RFID reader, have a look at
`../terminal-case`

Feature overview
----------------

   - Sized so that it can be sandwiched with a RFID-RC522 board (40mmx60mm),
     including the same mounting holes.
   - Communication via RS232, using standard RJ45 plug.
   - Separate 4-pin header to connect without RJ45
   - In-circuit programming header.
   - Uses Atmega8 in PDIP cases, mostly because I had a bunch lying around :)
   - Allows to connect RFID-RC522, 4x3 keypad, HD44780 LCD, RGB-LED,
     speaker-buzzer. All supported by firmware.
   - Uses a [SP3232E][sp3232-spec], essentially a MAX232 compatible chip
     for 3.3V supply voltage and only 100nF capcitors needed.
   - Uses the SP3232E charge pump to get the negative voltage needed to drive
     LCD contrast.
   - Little 'breadboard area' for quick hacks.

Serial connection with RJ45
---------------------------

Pinout of RJ45 is a somewhat 'standard' way to connect an RJ45 with RS232
and is used in [Various][rj45-terminal-1] [router][rj45-terminal-2]
terminal connections (But note to whoever came up with this first probably 20
years ago: **bad choice**! Pin 3 and 6 are twisted together in an ethernet
cable, so this just optimized crosstalk between RX and TX...).

We also power the terminal circuit via the RJ45. We use the fact that the
lines `DTR` and `RTS` can always be set to +12V safely (within RS232 voltage
range) from the host (=[Data Terminal Equipment; DTE][DTE]). We use that to
provide power to our terminal (for any other endpoint with RJ45, this set-up
would look like a benign 'always ready' flow control signal; so does not damage
equipment). With two lines providing power, it should be possible to power
smallish loads such as an electric strike even over a longer line without too
much voltage drop.

The following list is the RJ45 connections from view of the terminal,
the [DCE side][DCE].
The *Line* in the following list represents the corresponding RJ45 pin.
Also as reference gives the [9-pin Sub-D connector (DB9)][db9-pinout]
equivalent connection on a 'standard' connector.
   - Line 1: Not connected (usually: DCE:RTS (out) / DTE:CTS (in)) *DB9-8*
   - Line 2: Not connected (usually: DCE:DTR (out) / DTE:DSR (in)) *DB9-1*
   - Line 3: **TxD**  (on host DTE:RxD) *DB9-2*
   - Line 4: **GND** (on host DTE:RI, 'Ring indicator') *DB9-5* (DB9-9). (Blue solid).
   - Line 5: **GND** (GND) *DB9-5* (Blue striped).
   - Line 6: **RxD** (on host DTE:TxD) *DB9-3*
   - Line 7: **12V** supply in (usually: DCE:DSR (in) / DTE:DTR (out), DB9-4; but not connected there, just constantly powered. (Brown striped).
   - Line 8: **12V** supply in (usually: DCE:CTS (in) / DTE:RTS (out), DB9-7; powered, dito) (Brown solid).

(Let's see how well RS232 works, if long lines create trouble, we might consider
RS422 physical).

Hacking DB9 to RJ45 connector
------------------------------
If you want to have a standard DB9-female to RJ45 connector work with
this terminal, you need to hack it: open it up, and *disconnect* the cables
that go to **DB4** and **DB7** and instead connect them to an external 12V
power supply, with ground connected to **DB5** (ideally, you'd provide some
current limiting like 500mA to the 12V line; so if things go wrong, not
catastrophically so).

Double check on the RJ45 connector, that line 4 and 5 are on ground,
Line 7 and 8 on +12V.

![PCB][pcb]

Assembly
--------
Watch out in this version of the board, the [LD1117] LDO has to be soldered
in differently than the board suggests.

Unfortunately, it has a different pintout than other voltage regulators, this
is why it slipped in. Instead of [`Vin`, `GND`, `Vout`], it is
[`GND`, `Vout`, `Vin`]. So you need to solder in some lead-twisting :)

Tip for the RC522 reader. Depending on how you sandwich it, the chrystal oscillator it
has on top pokes out to the front; it might make sense to resolder it and put on the back
of the board.

TODO
----
TODOs for future versions; learning from previous board

   - Switch to atmega168 with less bulky 32 TQFP and 4 more IO pins.
   - Fix pin-out of LDO (also, switch to ceramic capacitors for less
     board height)
   - In general optimize board for best stacking, e.g. avoid collision with
     the crystal on the RFID board.
   - Provide an extra fourth column for keypad.
   - separate connector for 4st row, 4th column for an optional door-bell.
   - Silkscreen label all the pins for RC522 to minimize confusion which way
     to stack.
   - separate out RGB LEDs from LCD (when more IO available)
   - Maybe provide a reverse 8x2 connector for LCD (some LCDs are better
     connected from the reverse side)
   - Provide additional 16x1 connector to simply connect these LCD types.
   - Directly mount RGB LED on bord for less mouting hassle
   - Add space for 754410 to possibly directly drive door-strikes.
   - Add space for extra transistor to amplify speaker output.
   - Add space for little buzzer on-board.


[pcb]: https://github.com/hzeller/rfid-access-control/raw/master/img/terminal-pcb.png
[sp3232-spec]: http://www.exar.com/common/content/document.ashx?id=619
[rj45-terminal-1]: http://www.allpinouts.org/index.php/Cisco_Console_RJ45_to_DB9_Pin
[rj45-terminal-2]: http://kb.juniper.net/InfoCenter/index?page=content&id=KB13272
[db9-pinout]: http://www.db9-pinout.com/
[DTE]: http://en.wikipedia.org/wiki/Data_terminal_equipment
[DCE]: http://en.wikipedia.org/wiki/Data_circuit-terminating_equipment
[LD1117]: http://www.st.com/web/en/resource/technical/document/datasheet/CD00000544.pdf
