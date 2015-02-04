package main

import (
	"time"
)

// Callback interface to be implemented to receive events generated
// by terminals, the little boxes mounted next to doors :)
// This is the interface that code should implement to interact with
// such a terminal - the Init() function will be called once and pass
// you a way to talk back to that terminal.
//
// Each method call should return quickly; if you need to do something
// dependent on time, implement HandleTick()
type TerminalEventHandler interface {
	// Initialize. This is called once in the beginning and gets passed the
	// TerminalStub connected to the terminal. This provides the interface
	// to trigger actions, e.g. activating LEDs or emitting a tone.
	Init(my_terminal Terminal)

	// Called when the connection to this EventHandler is shut down.
	HandleShutdown()

	// HandleKeypress receives each character typed on the keypad.
	// These are ASCII encoded bytes in the range '0'..'9' and '*' and '#'.
	HandleKeypress(byte)

	// HandleRFID receives the ID of an RFID card presented to the
	// terminal. While the card is held in front of the terminal, this
	// repeats every couple of 100ms.
	HandleRFID(string)

	// HandleTick is called roughly every 500ms when idle.
	HandleTick()
}

// The API to interact with the terminal. If you implement a
// TerminalEventHandler, you get your corresponding terminal object passed in
// Init().
type Terminal interface {
	// Get the name of the terminal.
	GetTerminalName() string

	// Show the LED color. String contains a string with a combination of
	// characters 'R', 'G', 'B'. So ShowColor("RG") would show yellow for
	// instance. Empty string: LEDs off.
	ShowColor(colors string)

	// Buzz the speaker. Tone code can be 'H' or 'L' for high or low
	// frequency (TODO: that should probably be some enum);
	// "duration" does this for the given duration.
	BuzzSpeaker(toneCode string, duration time.Duration)

	// Write to the LCD. The "row" is the row to write to (starting with
	// 0). The "text" is the line to be written.
	WriteLCD(row int, text string)
}
