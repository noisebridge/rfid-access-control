package main

import (
	"bufio"
	"fmt"
	"github.com/tarm/goserial"
	"io"
	"log"
	"os"
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
	serialFile    io.ReadWriteCloser
	inputLineChan chan string
}

func (t *TerminalStub) Run(handler Handler) {
	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 38400}
	var err error
	t.serialFile, err = serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
	t.inputLineChan = make(chan string)
	go t.readLineLoop()
	handler.Init(t)
	for {
		line := <-t.inputLineChan
		switch {
		case line[0] == '#':
			// ignore
		case line[0] == 'I':
			handler.HandleRFID(line[1:])
		case line[0] == 'K':
			handler.HandleKeypress(line[1])
		case len(line) == 0:
			handler.HandleTick()
		default:
			fmt.Println("Unexpected input: ", line)
		}
	}
}

func (t *TerminalStub) WriteLCD(line int, text string) bool {
	t.writeLine(fmt.Sprintf("M%d%s", line, text))
	result := <- t.inputLineChan
	success := result[0] == 'M'
	if !success {
		fmt.Println("LCD write error:", result)
	}
	return success;
}

func (t *TerminalStub) readLineLoop() {
	reader := bufio.NewReader(t.serialFile)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "reading input:", err)
		}
		t.inputLineChan <- line
	}
}

func (t *TerminalStub) writeLine(line string) {
	fmt.Println("Sending ", line)
	_, err := t.serialFile.Write([]byte(line + "\n"))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	t := new(TerminalStub)
	handler := new(DebugHandler)
	t.Run(handler)
}
