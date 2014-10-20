package main

import (
	"bufio"
	"fmt"
	"github.com/tarm/goserial"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

// Callback interface to be implemented to receive events generated
// by terminals.
// Each method call should return quickly; if you need to do something
// dependent on time, implement HandleTick()
type Handler interface {
	// Initialize. This is called once in the beginning and gets the
	// TerminalStub connected to the terminal. This allows to trigger
	// actions, such as writing to the LCD display.
	Init(t *TerminalStub)

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

type TerminalStub struct {
	serialFile      io.ReadWriteCloser
	responseChannel chan string // Strings coming as response to requests
	eventChannel    chan string // Strings representing input events.
}

func NewTerminalStub(port string, baudrate int) *TerminalStub {
	t := new(TerminalStub)
	c := &serial.Config{Name: port, Baud: baudrate}
	var err error
	t.serialFile, err = serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
	t.eventChannel = make(chan string, 2)
	t.responseChannel = make(chan string)
	go t.readLineLoop()
	return t
}

func (t *TerminalStub) Run(handler Handler) {
	handler.Init(t)
	for {
		line := <-t.eventChannel
		switch {
		case line[0] == 'I':
			handler.HandleRFID(line[1:])
		case line[0] == 'K':
			handler.HandleKeypress(line[1])
		case len(line) == 0:
			handler.HandleTick()
		default:
			log.Print("Unexpected input: ", line)
		}
	}
}

// Ask the terminal about its name.
func (t *TerminalStub) GetTerminalName() string {
	t.writeLine("n")
	result := <-t.responseChannel
	success := (result[0] == 'n')
	if !success {
		log.Print("name receive problem:", result)
	}
	return result[1:]
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

func (t *TerminalStub) BuzzSpeaker() { }

func (t *TerminalStub) readLineLoop() {
	reader := bufio.NewReader(t.serialFile)
	for {
		// TODO: select with 500ms timeout. on timeout, send
		// empty line to event, otherwise do readline.
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
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr,
			"usage: %s <serial-device>[:baudrate] [<serial-device>[:baudrate]...]\n",
			os.Args[0])
		return
	}

	authenticator := NewAuthenticator("", "/var/access/legacy_keycode.txt")
	//a := NewAuthenticator("users.csv", "legacy.txt")
	//log.Println("Code 99a9 has access?", a.AuthUser("99a9"))
	//log.Println("Code a9f031 has access?", a.AuthUser("a9f031"))
	//return;

	for i, arg := range os.Args {
		if i == 0 {
			continue
		}

		devicepath, baudrate := parseArg(arg)
		t := NewTerminalStub(devicepath, baudrate)
		t.GetTerminalName()
		log.Printf("Device '%s' connected to '%s'", arg, t.GetTerminalName())
		handler := NewAccessHandler(authenticator)
		t.Run(handler)
	}
}
