package main

import (
	"github.com/tarm/goserial"
//	"github.com/hzeller/rfid-access-control/software/earl"
	"os"
	"log"
	"bufio"
	"fmt"
	"io"
)

type Handler interface {
	Init(t *TerminalStub)
	HandleKeypress(byte)
	HandleRFID(string)
	HandleTick()
}

type TerminalStub struct {
	serialFile io.ReadWriteCloser
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
	go t.ReadLineLoop()
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

func (t *TerminalStub) ReadLineLoop() {
	reader := bufio.NewReader(t.serialFile)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "reading input:", err)
		}
		t.inputLineChan <- line
	}
}

func (t *TerminalStub) WriteLine(line string) {
	fmt.Println("Sending ", line)
	_, err := t.serialFile.Write([]byte(line+"\n"))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	t := new(TerminalStub)
	handler := new(DebugHandler)
	t.Run(handler)
}
