Terminal PCB
============

   - Sized so that it can be sandwiched with a RFID-RC522 board (40mmx60mm),
     including the same mounting holes.
   - Communication via RS232, using standard RJ45 plug.
   - Powered on RJ45 with 12V. Pinout of RJ45
      - Line 3: **TX**
      - Line 4,5: **GND**
      - Line 6: **RX**
      - Line 7: **12V** in.
      - Line 1, 2, 8: Not Connected.
   - Separate 4-pin header to connect without RJ45
   - In-circuit programming header.
   - Uses Atmega8 in PDIP cases, mostly because I had a bunch lying around :)
   - Allows to connect RFID-RC522, 4x3 keypad, HD44780 LCD, 2 aux; supported
     by firmware.
   - Uses a [SP3232E][sp3232-spec], essentially a MAX232 compatible chip
     for 3.3V and only 100nF capcitors needed.
   - Uses the SP3232E charge pump to get the negative voltage needed to drive
     LCD contrast (untested)
   - Little 'breadboard area' for quick hacks.

![PCB][pcb]

[pcb]: https://github.com/hzeller/rfid-access-control/raw/master/img/terminal-pcb.png
[sp3232-spec]: http://www.exar.com/common/content/document.ashx?id=619
