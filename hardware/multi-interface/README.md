Multi serial line driver RJ45
=============================

Features
--------
   - 4 interfaces with RJ45 plugs (uses a combined 4-block of RJ45 I found in
     an old switch. Standard distance would need to pull these apart 50 thou/mil)
   - TTL compatible with 3.3V, so can interface with Rasberry Pi or
     Beaglebone Black.
   - Creates RS232 line voltages from a single 3.3V supply.
   - Uses a [SP3232E][sp3232-spec], essentially a MAX232 compatible chip
     for 3.3V supply voltage and only 100nF capcitors needed.
   - Allows to power external terminals on Line 7 and 8 (see `../terminal` for
     details)
   - FYI M3 mounting holes are 18mmx64mm apart.

![PCB][pcb]

TODO
----
Learnings from first board, possibly add to second revision

   - It would be useful to have a four line voltage/ground connector, directly
      compatible with the corresponding 4-pin connector on the terminal side.
   - Silk-screen marking RX/TX
   - 3-Pin connector for 12v: [GND-12v-GND] to allow turn-agnostic plug.
   - Possibe protection circuit: reverse polarity protection diode; PTC fuse
   - optional 12V power LED ? Maybe optional 3.3V power LED
   - RX/TX LEDs. Check connection of RJ45 plugs, some have built-in LEDs
     (usually to indicate Ethernet 100Mbps/1Gbps), would be cool to re-use
     these. Also optional space for discrete diodes.
   - All LEDs: also provide pads on the bottom of the board if that is the
     part visible.
    
[pcb]: https://github.com/hzeller/rfid-access-control/raw/master/img/multi-interface-pcb.png
[sp3232-spec]: http://www.exar.com/common/content/document.ashx?id=619
