<!-- -*- mode: markdown; indent-tabs-mode: nil; -*- -->
Earl - Host Software
====================

Software running on the host computer.
Running on an Raspberry Pi to access some GPIO pins and multiple serial
interfaces.

Language
--------
Written in Go. Mostly to learn a new language. Also it has the simplicity of
Python without the ugliness of Python.

The previous version of this software was called `baron`. This being the next
version, it is called `earl`.

Compile
-------
(TODO: mostly placeholder. To be filled with correct information)
To get going, install go if you haven't already:

     sudo aptitude install golang

Set your environment variable `GOPATH` to some directory where you would
like to have your [go workspace][golang-gopath]. That is the scratch space
where go puts build artifacts and dependent libraries.

     export GOPATH=~/go-root    # Good idea to put that in your ~/.bashrc
     mkdir $GOPATH

(Also, on the Raspberry Pi, you want to set the environment variable `GOARM=5`,
otherwise binaries don't run.)

Ok, back to the `rfid-access-control/software/earl` directory.

     go get       # Only do this the first time. Get needed serial library.
     
     make         # Builds binary, runs tests
     
     # Installing. Like everything running as root, you first want to see what
     # the following command is doing. So let's do a dry-run
     make -n install

     # Alright, ready for the real thing
     sudo make install # install binary and init.d script

Hacking
-------
Adding some code that deals with a serial terminal is simple. The low-level
handling of terminals is taken care of for you. In order to
interact with a terminal, you need to implement a `TerminalEventHandler` to
receive events such as key-presses and RFID tokens. You'll get passed a
`Terminal` API that allows you to interact with the terminal (switching on LEDs
or writing to the LCD display and such).

Having said that, you probably don't have to implement anything for a new door
type as `AccessHandler` probably already does what you need.

The `Authenticator` is the interface that implements the API to authenticate
users. Also user change operations are implemented (which in itself it requires
to be authenticated). The implementation, the `FileBasedAuthenticator` is storing
its state in a simple (possibly hand-editable) flat CSV file.

The interesting stuff interacting with the access terminals is implemented
in `accesshandler.go`. In `authenticator.go`, there is the ACL file handling.
The LCD frontend stuff is implemented in `uicontrolhandler.go`.

Interfaces
----------
** Serial interface
(TODO: describe serial interfaces connected. Currently: the build-in /dev/AMA0
and two USB serial adapters. Describe that it does not matter where to connect
any terminal, they find their place)

** Relays
(TODO: describe connection to GPIO pins)

** RPi On-board Serial Interface

If you want to use the `/dev/AMA0` serial interface of the Raspberry Pi, make sure
to comment out the getty line in `/etc/inittab` that grabs that interface.
Otherwise things don't work smoothly :)

Features
--------
Features so far.

   - Talk the serial interface provided by the access terminal and
     its firmware (see these directories).
   - Should read gate PIN numbers from a file as read by previous access
     control system Baron ( https://github.com/noisebridge/noisebridge-baron )
   - Have another file with RFID numbers. Also this contains flags
     saying which user can add other users (Members can)
   - Both files should be re-loaded whenver they change externally
     (i.e. edit)
   - Multiple terminals can be connected to various ttys whose paths are
     given on the commandline. Internally, the program queries the name of the
     terminal to associate file-descriptor with physical terminal (thus,
     swapping the serial lines is not a problem). There is a handler for each
     of the named terminals; each of them might do different things.
   - There are 2 relay contacts connected to the Raspberry Pi that
     can be used to open gates. This happens via GPIO pins via the
     file `/sys/...` interface as we don't need speed.
   - There are 4 terminals to be handled by this software
     (probably later more)
     Each of them does a little bit different things, they should be implemented
     in a simple way (so: someone only needs to implement a handler), with the
     complicated stuff (serial line and such) handled in the background.
     Note, each of these have a human readable name (see firmware help how to
     set it).
       - Downstairs gate. Reads PIN number or RFID. If match, gate is opened
         via one relay contact.
       - Upstairs door. Reads RFID. If match, upstairs gate is opened
         via other relay contact.
       - In-space terminal (probably inside next to
         the door). Has keypad, RFID reader and LCD display. Provides simple
         way to add new users, something like:
          1. show existing RFID card of Noisebridge member.
          2. ask to add user or update user.
          3. present user RFID card.
          4. new RFID card is added to the file (or time extended)
        User-interaction with keypad and LCD display.
        - TODO: allow to add temporary pins
        - TODO: provide a terminal interface
   - (_TBD_) Future: We might equip a terminal with an H-bridge to open the
     electric strike, thus relieving one of the relay contacts.
     That would be connected to the inside terminal at the door upstairs. The
     outside terminal receives an RFID request, and when it decides that this
     was ok, tells the _inside_ handler to open the strike. This needs to be
     handled in a thread-safe way as each terminal can only have one outstanding
     request at a time to avoid confusion. Initially: not needed.

[golang-gopath]: https://golang.org/doc/code.html#GOPATH
