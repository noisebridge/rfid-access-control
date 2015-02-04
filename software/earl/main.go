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
	idleTickTime                = 500 * time.Millisecond
)

// Physical actions triggered by earl activity
type PhysicalActions interface {
	OpenDoor(which Target) // Open strike for given door
	RingBell(which Target) // Inside space: generate audible tone.
}

type DoorbellUI interface {
	// Handle someone pressing the doorbell button or triggering doorbell
	// by swiping an RFID outside the user time.
	HandleDoorbell(which Target, message string)
}

type SimpleDoorbellUI struct {
	actions PhysicalActions
}

// Simplest case of doorbell UI: ring the bell.
func (d *SimpleDoorbellUI) HandleDoorbell(which Target, message string) {
	log.Printf("Doorbell %s : %s\n", which, message)
	// TODO: rate-limiting for noisy ringers.
	d.actions.RingBell(which)
}

type TerminalImpl struct {
	serialFile      io.ReadWriteCloser
	responseChannel chan string // Strings coming as response to requests
	eventChannel    chan string // Strings representing input events.
	errorState      bool
	name            string             // The name of the terminal e.g. 'upstairs'
	lastLCDContent  [maxLCDRows]string // last content sent to lcd
	logPrefix       string
}

func NewTerminalImpl(port string, baudrate int) (*TerminalImpl, error) {
	t := &TerminalImpl{
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
		t.shutdown()
		return nil, errors.New("Couldn't get name of terminal.")
	}
	return t, nil
}

// Public 'Terminal' interface
func (t *TerminalImpl) GetTerminalName() string {
	return t.name
}

func (t *TerminalImpl) WriteLCD(line int, text string) {
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
func (t *TerminalImpl) BuzzSpeaker(toneCode string, duration time.Duration) {
	t.sendAndAwaitResponse(fmt.Sprintf("T%s%d", toneCode, int64(duration/time.Millisecond)))
}

func (t *TerminalImpl) ShowColor(colors string) {
	t.sendAndAwaitResponse(fmt.Sprintf("L%s", colors))
}

// Read data coming from the terminal and stuff it into the right
// channels (we distinguish responses of commands from event notifications)
func (t *TerminalImpl) inputScanLoop() {
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
			// These are events sent asynchronously from the
			// terminal to signify incoming key-presses or RFID
			// reads
			t.eventChannel <- line
		default:
			// Everything else coming from the terminal is in
			// response to something we requested.
			t.responseChannel <- line
		}
	}
}

// Line-level interaction with the terminal. The protocol encodes
// the command as the first character, and the reply of the terminal
// (which arrives in the responseChannel) echos that character as first char.
// If that is not the case, we're in some error condition.
// This function sends the request and verifies that the response
// is as expected.
func (t *TerminalImpl) sendAndAwaitResponse(toSend string) string {
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
	case <-time.After(2 * time.Second):
		// Terminal should've returned immediately. Timeout: bad.
		t.errorState = true
		return ""
	}
	return "" // make old compiler happy
}

// Blow out the tubes.
func (t *TerminalImpl) discardInitialInput() {
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

// Run until we encounter an IO problem or we can't verify to be
// connected anymore.
func (t *TerminalImpl) runEventLoop(handler TerminalEventHandler) {
	var tick_count uint32
	lastTickTime := time.Now()
	handler.Init(t)
	for !t.errorState {
		// If the events come in very quickly, the idle tick might
		// be starved. So make sure to inject some.
		if time.Now().Sub(lastTickTime) > 4*idleTickTime {
			handler.HandleTick()
			lastTickTime = time.Now()
		}
		select {
		case line := <-t.eventChannel:
			switch {
			case line[0] == 'I':
				if rfid, ok := t.parseRFIDResponse(line); ok {
					handler.HandleRFID(rfid)
				}
			case line[0] == 'K':
				handler.HandleKeypress(line[1])
			default:
				log.Printf("%s: Unexpected input '%s'", t.logPrefix, line)
			}

		case <-time.After(idleTickTime):
			handler.HandleTick()
			lastTickTime = time.Now()
			tick_count++
			if tick_count%10 == 0 && !t.verifyConnected() {
				return
			}
		}
	}
}

func (t *TerminalImpl) parseRFIDResponse(from_terminal string) (string, bool) {
	// The ID comes as "<length> <code>". Get the code.
	rfid_elements := strings.Split(from_terminal, " ")
	if len(rfid_elements) != 2 {
		return "", false
	}
	got_len, _ := strconv.Atoi(rfid_elements[0]) // number of bytes
	rfid := strings.TrimSpace(rfid_elements[1])  // bytes as hex
	if len(rfid) > 0 && len(rfid) == 2*got_len {
		return rfid, true
	} else {
		return "", false
	}
}

// Regularly confirm that we are still connected to same terminal
// i.e. if connectors are disconnected or plugged around.
func (t *TerminalImpl) verifyConnected() bool {
	new_name := t.requestName()
	if t.errorState {
		log.Printf("%s: Error pinging terminal '%s'",
			t.logPrefix, t.name)
		return false
	}
	if new_name != t.name {
		log.Printf("%s: Name change ('%s', was '%s')",
			t.logPrefix, new_name, t.name)
		return false
	}
	return true
}

func (t *TerminalImpl) shutdown() {
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
func (t *TerminalImpl) requestName() string {
	result := t.sendAndAwaitResponse("n")
	if result == "" {
		return ""
	}
	return strings.TrimSpace(result[1:])
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
	authenticator   Authenticator
	physicalActions PhysicalActions
	doorbellUI      DoorbellUI
}

func HandleSerialDevice(devicepath string, baud int, backends *Backends) {
	var t *TerminalImpl
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

		t, _ = NewTerminalImpl(devicepath, baud)
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
			handler = NewAccessHandler(backends)

		case TargetControlUI:
			handler = NewControlHandler(backends)

		default:
			log.Printf("%s:%d: Terminal with unrecognized name '%s'",
				devicepath, baud, t.GetTerminalName())
		}

		if handler != nil {
			connect_successful = true
			retry_time = initialReconnectOnErrorTime
			log.Printf("%s:%d: connected to '%s'",
				devicepath, baud, t.GetTerminalName())
			t.runEventLoop(handler)
			handler.HandleShutdown()
		}
		t.shutdown()
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

	actions := NewGPIOActions()
	backends := &Backends{
		authenticator:   NewFileBasedAuthenticator(*userFileName),
		physicalActions: actions,
		doorbellUI:      &SimpleDoorbellUI{actions: actions},
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
