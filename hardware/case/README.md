The case for the RFID reader.
=============================

   - Mounting holes for the wall on the base-plate, invisible from top.
   - Cleat system allows for single-screw fixing of outer shell to wall.
   - Cabeling behind the board
   - Noisebridge Logo lends itself to a nice RFID-logo modification :)

![Case][case-image]

The case comes in two pieces, the base and the top case. The base is mounted
permanently to the wall (with three drywall-screws), the top case slides on
top of it and is held in place almost invisible with a single mounting screw
at the bottom.

Even without that mounting screw, gravity holds the case in place.
The mounting screw pulls down the inner assembly and thus firmly pulls it towards
the wall.
Removing that screw allows to slide the case off easily moving slightly upwards.

This 'xray' view reveals the 'french cleat' system that does the magic.
![On the inside][xray]

Hacking
-------
The case is written in OpenSCAD. The code could use a run of cleanup now that
it is clear how the result should look like :)

To get he logo, you have to first generate the dxf file from svg:

     make Noisebridge-logo.dxf


[case-image]: https://github.com/hzeller/rfid-access-control/raw/master/img/rfid-reader-case.png
[xray]: https://github.com/hzeller/rfid-access-control/raw/master/img/rfid-reader-xray-view.png


