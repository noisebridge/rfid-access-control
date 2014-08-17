Terminal PCB
============

The Access terminal mounted at the outside of the door. Typically connecting the
RFID reader (via SPI interface), but also a 4x3 keypad (or just simple door-bell
button) and an LCD can be connected. If wanted, even an electric strike via
some external H-bridge.

This board provides the necessary breakouts to connect the peripherals to an
Atmega8 and holds the MAX232 compatible line driver.
The software in `../../software/firmware` makes it useful.

For a simple housing of this board and the RFID reader, have a look at `../case`

   - Sized so that it can be sandwiched with a RFID-RC522 board (40mmx60mm),
     including the same mounting holes.
   - Communication via RS232, using standard RJ45 plug.
   - Powered on RJ45 with 12V: We use the fact that lines `DTR` and `RTS` can
     safely always be set to +12V from the host (=Data Terminal Equipment; DTE).
     We use that to provide power to our terminal.
     Pinout of RJ45 is a somewhat 'standard' way to connect an RJ45 with RS232
     and is used in [Various][rj45-terminal-1] [router][rj45-terminal-2]
     terminal connections.
     (But note to whoever came up with this first probably 20 years ago:
     **bad choice**! Pin 3 and 6 are twisted together in an ethernet cable, so
     this optimized crosstalk between RX and TX. How .. why ... I don't even ..).
     RJ45 from view of the terminal, the DCE side
     (=data circuit-terminating equipment; DCE).
       - Line 1: Not connected (usually: DCE:RTS (out) / DTE:CTS (in)) DB9-8
       - Line 2: Not connected (usually: DCE:DTR (out) / DTE:DSR (in)) DB9-1
       - Line 3: **TxD**  (on host DTE:RxD) DB9-2
       - Line 4: **GND** (on host DTE:RI, 'Ring indicator') DB9-9/DB9-5
       - Line 5: **GND** (GND) DB9-5
       - Line 6: **RxD** (on host DTE:TxD) DB9-3
       - Line 7: **12V** supply in ('standard': DTE:DTR (out)) DB9-4
       - Line 8: **12V** supply in ('standard': DTE:RTS (out)) DB9-7
    (Let's see how well RS232 works, if lines too long, consider RS422 physical).
   - Separate 4-pin header to connect without RJ45
   - In-circuit programming header.
   - Uses Atmega8 in PDIP cases, mostly because I had a bunch lying around :)
   - Allows to connect RFID-RC522, 4x3 keypad, HD44780 LCD, 2 aux; supported
     by firmware.
   - Uses a [SP3232E][sp3232-spec], essentially a MAX232 compatible chip
     for 3.3V supply voltage and only 100nF capcitors needed.
   - Uses the SP3232E charge pump to get the negative voltage needed to drive
     LCD contrast (untested)
   - Little 'breadboard area' for quick hacks.

![PCB][pcb]

[pcb]: https://github.com/hzeller/rfid-access-control/raw/master/img/terminal-pcb.png
[sp3232-spec]: http://www.exar.com/common/content/document.ashx?id=619
[rj45-terminal-1] http://www.allpinouts.org/index.php/Cisco_Console_RJ45_to_DB9_Pin
[rj45-terminal-2] http://kb.juniper.net/InfoCenter/index?page=content&id=KB13272
