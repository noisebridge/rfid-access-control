Terminal PCB
============

   - Communication via RS232, via RJ45
   - Powered on line 7 of RJ45 with 12V
   - allows to connect RFID-RC522, 4x3 keypad, HD44780 LCD, 2 aux
   - uses a SP3232E, essentially a MAX232 compatible chip for 3.3V and only
     100nF capcitors needed.
   - Uses the charge pump SP3232E to get the negative voltage needed to drive
     LCD contrast (untested)

![PCB][pcb]

[pcb]: https://github.com/hzeller/rfid-access-control/raw/master/img/terminal-pcb.png
