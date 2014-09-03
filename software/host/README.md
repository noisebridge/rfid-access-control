Access Control Host Software
============================

Software running on the host computer.
Running on an Raspberry Pi to access some GPIO pins.

Language
--------
Written in Go. Mostly to learn a new language. Also it has the simplicity of
Python without the ugliness of Python.

Compile
-------
(TODO: mostly placeholder. To be filled with correct information)
To get going, install go

     sudo aptitude install golang

Set your environment variable `GOPATH` to this directory and type

     export GOPATH=`pwd`
     go get       # Only do this the first time. Get needed libraries.
     go install   # will copy the resulting binary into $GOLANG/bin

Features
--------
Ok, there are no features yet, at this point it is all spec.

   - Talk the serial interface provided by the access terminal and its firmware
     (see these directories).
   - Should read gate PIN numbers from a file as read by previous access
     control system Baron ( https://github.com/noisebridge/noisebridge-baron )
   - Have another file with RFID numbers. Also this contains flags saying which
     user can add other users. So probably two tab-separated columns.
   - Both files should be re-loaded whenver they change externally (i.e. edit)
   - Multiple terminals can be connected to various ttys whose paths come
     on the commandline. Internally, the program queries the name of the
     terminal to associate file-descriptor with physical terminal (thus,
     swapping the serial lines is not a problem). There is a handler for each
     of the named terminals; each of them might do different things.
   - There are 2 relay contacts connected to the Raspberry Pi that can be used
     to open gates. This happens via GPIO pins (TODO: access these via go;
     in the simplest case just via the file `/sys/...` interface as we don't
     need speed).
   - There are 4 terminals to be handled by this software (probably later more)
     Each of them does a little bit different things, they should be implemented
     in a simple way (so: someone only needs to implement a handler), with the
     complicated stuff (serial line and such) handled in the background.
     Note, each of these have a human readable name (see firmware help how to
     set it).
       - Downstairs gate. Reads PIN number. If match, gate is opened via one
         relay contact.
       - Upstairs door. Reads RFID. If match, upstairs gate is opened via other
         relay contact.
       - In-space terminal (probably inside next to the door). Has keypad,
         RFID reader and LCD display. Provides simple way to add new users:
          1 show existing RFID card of 'deciding member'
	  2 ask to add user
	  3 present new RFID card.
	  4 new RFID card is added to the file.
	 User-interaction with keypad and LCD display.
      - Elevator door. Later.
   - Future: We might equip a terminal with an H-bridge to open the electric
     strike, thus relieving one of the relay contacts.
     That would be connected to the inside terminal at the door upstairs. The
     outside terminal receives an RFID request, and when it decides that this
     was ok, tells the _inside_ handler to open the strike. This needs to be
     handled in a thread-safe way as each terminal can only have one outstanding
     request at a time to avoid confusion. Initially: not needed.
