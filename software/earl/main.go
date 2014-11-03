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

// Each access point has their own name. The terminals can identify
// by that name.
type Target string // TODO: find better name for this type
const (
	TargetDownstairs = Target("gate")
	TargetUpstairs   = Target("upstairs")
	TargetElevator   = Target("elevator")
	TargetControlUI  = Target("control") // UI to add new users.
	// Someday we'll have the network closet locked down
	//TargetNetwork = "closet"
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
	WriteLCD(row int, text string)
}

// Callback interface to be implemented to receive events generated
// by terminals.
// Each method call should return quickly; if you need to do something
// dependent on time, implement HandleTick()
type TerminalEventHandler interface {
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
type DoorActions interface {
	OpenDoor(which Target)
}

type TerminalStub struct {
	serialFile      io.ReadWriteCloser
	responseChannel chan string // Strings coming as response to requests
	eventChannel    chan string // Strings representing input events.
	name            string      // The name of the terminal e.g. 'upstairs'
	lastLCDContent  [2]string   // last content sent to lcd
}

func NewTerminalStub(port string, baudrate int) *TerminalStub {
	t := new(TerminalStub)
	c := &serial.Config{Name: port, Baud: baudrate}
	var err error
	t.serialFile, err = serial.OpenPort(c)
	if err != nil {
		log.Println(err.Error())
		return nil
	}
	t.eventChannel = make(chan string, 10)
	t.responseChannel = make(chan string, 10)
	go t.readLineLoop()
	return t
}

func (t *TerminalStub) Run(handler TerminalEventHandler) {
	handler.Init(t)
	for {
		select {
		case line := <-t.eventChannel:
			switch {
			case line[0] == 'I':
				handler.HandleRFID(line[1:])
			case line[0] == 'K':
				handler.HandleKeypress(line[1])
			default:
				log.Print("Unexpected input: ", line)
			}
		case <-time.After(500 * time.Millisecond):
			handler.HandleTick()
		}
	}
}

// Ask the terminal about its name.
func (t *TerminalStub) loadTerminalName() {
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

func (t *TerminalStub) WriteLCD(line int, text string) {
	if line < 0 || line >= len(t.lastLCDContent) {
		return
	}
	// Only send line if it is different from what is shown already.
	// TODO: too long lines: scroll back and forth.
	newContent := fmt.Sprintf("M%d%s", line, text)
	if t.lastLCDContent[line] == newContent {
		return
	}
	t.writeLine(newContent)
	t.lastLCDContent[line] = newContent
	_ = <-t.responseChannel
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
	//log.Print("Sending ", line)
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
	userFileName := flag.String("users", "/var/access/users.csv", "User Authentication file.")
	legacyFileName := flag.String("users-legacy", "/var/access/legacy_keycode.txt", "Legacy Gate-PIN file")
	logFileName := flag.String("logfile", "", "The log file, default = stdout")
	addNameOnConsole := flag.Bool("console-name", false, "Provide new-user name on console. Only works if tty attached to process")

	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintf(os.Stderr,
			"usage: %s [options] <serial-device>[:baudrate] [<serial-device>[:baudrate]...]\nOptions\n",
			os.Args[0])
		flag.PrintDefaults()
		return
	}

	if *logFileName != "" {
		logfile, err := os.OpenFile(*logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Error opening log file", err)
		}
		defer logfile.Close()
		log.SetOutput(logfile)
	}

	log.Println("Starting...")

	authenticator := NewFileBasedAuthenticator(*userFileName, *legacyFileName)
	doorActions := new(GPIOActions)
	doorActions.Init()

	for _, arg := range flag.Args() {
		devicepath, baudrate := parseArg(arg)
		t := NewTerminalStub(devicepath, baudrate)
		if t == nil {
			// TODO: handle reconnect.
			log.Printf("Couldn't connect: %s", arg)
			continue
		}
		t.loadTerminalName() // Need to spam this a few times to reset the device
		t.loadTerminalName()
		log.Printf("Device '%s' connected to '%s'", arg, t.GetTerminalName())
		// Terminals are dispatched by name. There might be different handlers
		// for the name e.g. handlers that deal with reading codes and opening
		// doors, but also the UI handler dealing with adding new users.
		var handler TerminalEventHandler
		switch Target(t.GetTerminalName()) {
		case TargetDownstairs, TargetUpstairs, TargetElevator:
			handler = NewAccessHandler(authenticator, doorActions)

		case TargetControlUI:
			handler = NewControlHandler(authenticator, *addNameOnConsole)

		default:
			log.Printf("Don't know how to deal with terminal '%s'", t.GetTerminalName())
		}

		if handler != nil {
			t.Run(handler)
		}
	}
}
