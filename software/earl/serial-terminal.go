package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/tarm/goserial"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
)

type SerialTerminal struct {
	serialFile      io.ReadWriteCloser
	responseChannel chan string // Strings coming as response to requests
	eventChannel    chan string // Strings representing input events.
	errorState      bool
	name            string             // The name of the terminal e.g. 'upstairs'
	lastLCDContent  [maxLCDRows]string // last content sent to lcd
	logPrefix       string
}

func NewSerialTerminal(port string, baudrate int) (*SerialTerminal, error) {
	t := &SerialTerminal{
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

// Deliver events received from the hardware to the TerminalEventHandler.
// Run until we encounter an IO problem or we can't verify to be
// connected anymore. So the only reason for this loop exiting would be
// an error condition.
func (t *SerialTerminal) RunEventLoop(handler TerminalEventHandler) {
	var tick_count uint32
	lastTickTime := time.Now()
	handler.Init(t)
	defer handler.HandleShutdown()
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

// Public 'Terminal' interface
func (t *SerialTerminal) GetTerminalName() string {
	return t.name
}

func (t *SerialTerminal) WriteLCD(line int, text string) {
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

// Tell the buzzer to buzz. If toneCode should be 'H' or 'L'
func (t *SerialTerminal) BuzzSpeaker(toneCode string, duration time.Duration) {
	t.sendAndAwaitResponse(fmt.Sprintf("T%s%d", toneCode, int64(duration/time.Millisecond)))
}

func (t *SerialTerminal) ShowColor(colors string) {
	t.sendAndAwaitResponse(fmt.Sprintf("L%s", colors))
}

// Read data coming from the terminal and stuff it into the right
// channels (we distinguish responses of commands from event notifications)
func (t *SerialTerminal) inputScanLoop() {
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
func (t *SerialTerminal) sendAndAwaitResponse(toSend string) string {
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
func (t *SerialTerminal) discardInitialInput() {
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

func (t *SerialTerminal) parseRFIDResponse(from_terminal string) (string, bool) {
	// The ID comes as "<length> <code>". Get the code.
	rfid_elements := strings.Split(from_terminal[1:], " ")
	if len(rfid_elements) != 2 {
		return "", false
	}
	got_len, _ := strconv.Atoi(rfid_elements[0]) // number of bytes
	rfid := strings.TrimSpace(rfid_elements[1])  // bytes as hex
	if len(rfid) > 0 && len(rfid) == 2*got_len {
		return rfid, true
	}
	return "", false
}

// Regularly confirm that we are still connected to same terminal
// i.e. if connectors are disconnected or plugged around.
func (t *SerialTerminal) verifyConnected() bool {
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

func (t *SerialTerminal) shutdown() {
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
func (t *SerialTerminal) requestName() string {
	result := t.sendAndAwaitResponse("n")
	if result == "" {
		return ""
	}
	return strings.TrimSpace(result[1:])
}
