package main

import (
	"bufio"
	"errors"
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
)

const (
	maxLCDRows                  = 2
	maxLCDCols                  = 24
	defaultBaudrate             = 9600
	initialReconnectOnErrorTime = 2 * time.Second
	maxReconnectOnErrorTime     = 60 * time.Second
)

// Interacting with the terminal.
// The terminal does send as well asynchronous
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
	errorState      bool
	name            string             // The name of the terminal e.g. 'upstairs'
	lastLCDContent  [maxLCDRows]string // last content sent to lcd
	logPrefix       string
}

func NewTerminalStub(port string, baudrate int) (*TerminalStub, error) {
	t := &TerminalStub{
		errorState: false,
		logPrefix:  fmt.Sprintf("%s:%d", port, baudrate),
	}
	c := &serial.Config{Name: port, Baud: baudrate}
	var err error
	t.serialFile, err = serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	t.eventChannel = make(chan string, 10)
	t.responseChannel = make(chan string, 10)
	go t.inputScanLoop()
	t.discardInitialInput()
	t.name = t.requestName()
	if t.errorState {
		t.Shutdown()
		return nil, errors.New("Couldn't get name of terminal.")
	}
	return t, nil
}

// Blow out the tubes.
func (t *TerminalStub) discardInitialInput() {
	// The first connect with the terminal might catch the line in some
	// strange state with undiscarded input, so just discard that here
	// until we see a couple of 100ms of silence.
	// Also send one dummy request to properly blow out the TX-line
	// (whose response is discarded as well)
	t.serialFile.Write([]byte("n\n")) // dummy request for name
	select {
	case <-t.eventChannel: // discard
	case <-t.responseChannel: // discard
	case <-time.After(1000 * time.Millisecond):
		break
	}
}

// Run until we encounter an IO problem or the terminal name changed
// (e.g. due to plug-swapping)
func (t *TerminalStub) RunEventLoop(handler TerminalEventHandler) {
	var tick_count uint32
	handler.Init(t)
	for !t.errorState {
		select {
		case line := <-t.eventChannel:
			switch {
			case line[0] == 'I':
				handler.HandleRFID(line[1:])
			case line[0] == 'K':
				handler.HandleKeypress(line[1])
			default:
				log.Printf("%s: Unexpected input '%s'", t.logPrefix, line)
			}

		case <-time.After(500 * time.Millisecond):
			handler.HandleTick()
			tick_count++
			// Regularly confirm that we are still connected to same terminal
			// i.e. if connectors are disconnected or plugged around.
			if tick_count%10 == 0 {
				new_name := t.requestName()
				if t.errorState {
					log.Printf("%s: Error pinging terminal '%s'",
						t.logPrefix, t.name)
					return
				}
				if new_name != t.name {
					log.Printf("%s: Name change ('%s', was '%s')",
						t.logPrefix, new_name, t.name)
					return
				}
			}
		}
	}
}

func (t *TerminalStub) Shutdown() {
	// Not logging to not trash SD card.
	//log.Printf("%s: Shutdown '%s'", t.logPrefix, t.GetTerminalName())
	t.errorState = true

	// TODO: ideally, we want a clean shutdown of the reader
	// in inputScanLoop() which is blocking at this moment.
	// We would like to send it a message telling to stop
	// reading and closing the channel.
	// However, this doesn't work: reader.ReadString() is blocking and
	// we can't select on it, thus also not a way to select
	// in parallel on some <-shutdownRequested channel.
	// The only chance I see is to close the channel here and
	// expect the Read() to return with an error (it does not,
	// immediately,  so the ReaderWriterCloser in the serial package
	// has to be adapted).
	// Maybe there is a better solution ?
	t.serialFile.Close()
}

// Ask the terminal about its name. Returns true if we ran into a timeout.
func (t *TerminalStub) requestName() string {
	result := t.sendAndAwaitResponse("n")
	if result == "" {
		return ""
	}
	return strings.TrimSpace(result[1:])
}

func (t *TerminalStub) GetTerminalName() string {
	return t.name
}

func (t *TerminalStub) WriteLCD(line int, text string) {
	if line < 0 || line >= maxLCDRows {
		return
	}
	if len(text) > maxLCDCols {
		// TODO: too long lines: scroll back and forth.
		text = text[:maxLCDCols]
	}
	// Only send line if it is different from what is shown already.
	newContent := fmt.Sprintf("M%d%s", line, text)
	if t.lastLCDContent[line] == newContent {
		return
	}
	t.sendAndAwaitResponse(newContent)
	t.lastLCDContent[line] = newContent
}

//Tell the buzzer to buzz. If toneCode should be 'H' or 'L'
func (t *TerminalStub) BuzzSpeaker(toneCode string, duration time.Duration) {
	t.sendAndAwaitResponse(fmt.Sprintf("T%s%d", toneCode, int64(duration/time.Millisecond)))
}

func (t *TerminalStub) ShowColor(colors string) {
	t.sendAndAwaitResponse(fmt.Sprintf("L%s", colors))
}

// Read data coming from the terminal and stuff it into the right
// channels (we distinguish responses of commands from event notifications)
func (t *TerminalStub) inputScanLoop() {
	reader := bufio.NewReader(t.serialFile)
	for !t.errorState {
		line, err := reader.ReadString('\n')
		if err != nil {
			if !t.errorState {
				log.Printf("%s: reading input: %v", t.logPrefix, err)
			}
			t.errorState = true
			return
		}
		switch line[0] {
		case '#', 0:
			// ignore comment lines and obvious garbage.
		case 'I', 'K':
			t.eventChannel <- line
		default:
			t.responseChannel <- line
		}
	}
}

// Line-level interaction with the terminal. The protocol encodes
// the command as the first character, and the reply of the terminal
// echos that character as first character of its response.
// If that is not the case, we're in some error condition.
// This function sends the request and verifies that the response
// is as expected.
func (t *TerminalStub) sendAndAwaitResponse(toSend string) string {
	//log.Printf("%s Sending ", t.logPrefix, toSend)
	_, err := t.serialFile.Write([]byte(toSend + "\n"))
	if err != nil {
		t.errorState = true
		return ""
	}

	select {
	case result := <-t.responseChannel:
		if result[0] == toSend[0] {
			return result
		} else {
			log.Printf("%s: Unexpected result. Expected '%c', got '%s'",
				t.logPrefix, toSend[0], result)
			t.errorState = true
			return ""
		}
	case <-time.After(2 * time.Second): // Terminal always returns immediately
		t.errorState = true
		return ""
	}
	return "" // make old compiler happy
}

func parseArg(arg string) (devicepath string, baudrate int) {
	split := strings.Split(arg, ":")
	devicepath = split[0]
	baudrate = defaultBaudrate
	if len(split) > 1 {
		var err error
		if baudrate, err = strconv.Atoi(split[1]); err != nil {
			panic(err)
		}
	}
	return
}

type Backends struct {
	authenticator Authenticator
	doorActions   DoorActions
}

func HandleSerialDevice(devicepath string, baud int, backends *Backends) {
	var t *TerminalStub
	connect_successful := true
	retry_time := initialReconnectOnErrorTime
	for {
		if !connect_successful {
			time.Sleep(retry_time)
			retry_time *= 2 // exponential backoff.
			if retry_time > maxReconnectOnErrorTime {
				retry_time = maxReconnectOnErrorTime
			}
		}

		connect_successful = false

		t, _ = NewTerminalStub(devicepath, baud)
		if t == nil {
			continue
		}

		// Terminals are dispatched by name. There are different handlers
		// for the name e.g. handlers that deal with reading codes
		// and opening doors, but also the UI handler dealing with
		// adding new users.
		var handler TerminalEventHandler
		switch Target(t.GetTerminalName()) {
		case TargetDownstairs, TargetUpstairs, TargetElevator:
			handler = NewAccessHandler(backends.authenticator, backends.doorActions)

		case TargetControlUI:
			handler = NewControlHandler(backends.authenticator)

		default:
			log.Printf("%s:%d: Terminal with unrecognized name '%s'",
				devicepath, baud, t.GetTerminalName())
		}

		if handler != nil {
			connect_successful = true
			retry_time = initialReconnectOnErrorTime
			log.Printf("%s:%d: connected to '%s'",
				devicepath, baud, t.GetTerminalName())
			t.RunEventLoop(handler)
		}
		t.Shutdown()
		t = nil
	}
}

func main() {
	userFileName := flag.String("users", "/var/access/users.csv", "User Authentication file.")
	logFileName := flag.String("logfile", "", "The log file, default = stdout")
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

	backends := &Backends{
		authenticator: NewFileBasedAuthenticator(*userFileName),
		doorActions:   NewGPIOActions(),
	}

	// For each serial interface, we run an indepenent loop
	// making sure we are constantly connected.
	for _, arg := range flag.Args() {
		devicepath, baudrate := parseArg(arg)
		go HandleSerialDevice(devicepath, baudrate, backends)
	}

	var block_forever chan bool
	<-block_forever
}
