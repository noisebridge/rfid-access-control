package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/tarm/goserial"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Interacting with the terminal. The terminal does send as well asynchronous
// information, reflected in the 'Handler' interface below.
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
	WriteLCD(row int, text string) bool
}

// Callback interface to be implemented to receive events generated
// by terminals.
// Each method call should return quickly; if you need to do something
// dependent on time, implement HandleTick()
type Handler interface {
	// Initialize. This is called once in the beginning and gets the
	// TerminalStub connected to the terminal. This allows to trigger
	// actions, such as writing to the LCD display.
	Init(t Terminal)

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

// Actions as result of the authentication decisions.
type Target string // TODO: find better name
type DoorActions interface {
	OpenDoor(which Target)
}

type TerminalStub struct {
	serialFile      io.ReadWriteCloser
	responseChannel chan string // Strings coming as response to requests
	eventChannel    chan string // Strings representing input events.
	name            string      // The name
}

func NewTerminalStub(port string, baudrate int) *TerminalStub {
	t := new(TerminalStub)
	c := &serial.Config{Name: port, Baud: baudrate}
	var err error
	t.serialFile, err = serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
	t.eventChannel = make(chan string, 10)
	t.responseChannel = make(chan string, 10)
	go t.readLineLoop()
	return t
}

func (t *TerminalStub) Run(handler Handler) {
	handler.Init(t)
	for {
		select {
		case line := <-t.eventChannel:
			switch {
			case line[0] == 'I':
				handler.HandleRFID(line[1:])
			case line[0] == 'K':
				handler.HandleKeypress(line[1])
			//case len(line) == 0:
			//	handler.HandleTick()
			default:
				log.Print("Unexpected input: ", line)
			}
		case <-time.After(500 * time.Millisecond):
			handler.HandleTick()
		}
	}
}

// Ask the terminal about its name.
func (t *TerminalStub) LoadTerminalName() {
	t.writeLine("n")
	result := <-t.responseChannel
	success := (result[0] == 'n')
	if !success {
		log.Print("name receive problem:", result)
	}
	t.name = strings.TrimSpace(result[1:])
}

func (t *TerminalStub) GetTerminalName() string {
	return t.name
}

func (t *TerminalStub) WriteLCD(line int, text string) bool {
	t.writeLine(fmt.Sprintf("M%d%s", line, text))
	result := <-t.responseChannel
	success := (result[0] == 'M')
	if !success {
		log.Print("LCD write error:", result)
	}
	return success
}

//Tell the buzzer to buzz. If toneCode should be 'H' or 'L'
func (t *TerminalStub) BuzzSpeaker(toneCode string, duration time.Duration) {
	t.writeLine(fmt.Sprintf("T%s%d", toneCode, int64(duration/time.Millisecond)))
	_ = <-t.responseChannel
}

func (t *TerminalStub) ShowColor(colors string) {
	t.writeLine(fmt.Sprintf("L%s", colors))
	_ = <-t.responseChannel
}

func (t *TerminalStub) readLineLoop() {
	reader := bufio.NewReader(t.serialFile)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Print("reading input:", err)
		}
		switch line[0] {
		case '#': // ignore comment lines.
		case 'I', 'K':
			t.eventChannel <- line
		default:
			t.responseChannel <- line
		}
	}
}

func (t *TerminalStub) writeLine(line string) {
	log.Print("Sending ", line)
	_, err := t.serialFile.Write([]byte(line + "\n"))
	if err != nil {
		log.Fatal(err)
	}
}

func parseArg(arg string) (devicepath string, baudrate int) {
	split := strings.Split(arg, ":")
	devicepath = split[0]
	baudrate = 9600
	if len(split) > 1 {
		var err error
		if baudrate, err = strconv.Atoi(split[1]); err != nil {
			panic(err)
		}
	}
	return
}

func main() {
	logFilePtr := flag.String("logfile", "", "The log file, default = stdout")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintf(os.Stderr,
			"usage: %s <serial-device>[:baudrate] [<serial-device>[:baudrate]...]\n",
			os.Args[0])
		return
	}

	if *logFilePtr != "" {
		logfile, err := os.OpenFile(*logFilePtr, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Error opening log file", err)
		}
		defer logfile.Close()
		log.SetOutput(logfile)
	}

	authenticator := NewAuthenticator("/var/access/users.csv", "/var/access/legacy_keycode.txt")
	doorActions := new(GPIOActions)

	for _, arg := range flag.Args() {
		devicepath, baudrate := parseArg(arg)
		t := NewTerminalStub(devicepath, baudrate)
		t.LoadTerminalName() // Need to spam this a few times to reset the device
		t.LoadTerminalName()
		log.Printf("Device '%s' connected to '%s'", arg, t.GetTerminalName())
		// Each terminal might have a different Handler to deal with.
		// They are dispatched by name, so that it doesn't matter which
		// serial interface they are connected to.
		var handler Handler
		switch t.GetTerminalName() {
		case "gate":
		case "upstairs":
			handler = NewAccessHandler(authenticator, doorActions)

		default:
			log.Printf("No Handler for '%s'", t.GetTerminalName())
		}

		if handler != nil {
			t.Run(handler)
		}
	}
}
