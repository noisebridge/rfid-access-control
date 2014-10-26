package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type DebugHandler struct {
	m      [2]string
	lineNo int
	t      *TerminalStub
}

func (h *DebugHandler) Init(t *TerminalStub) {
	h.t = t
}

func (h *DebugHandler) HandleKeypress(b byte) {
	log.Print("Received keypress: ", string(b))
	switch b {
	case '#':
		if len(h.m[h.lineNo]) > 0 {
			h.m[h.lineNo] = h.m[h.lineNo][0 : len(h.m[h.lineNo])-1]
		}
	case '*':
		h.lineNo ^= 1
	default:

		h.m[h.lineNo] += string(b)
	}
	h.t.WriteLCD(h.lineNo, h.m[h.lineNo])
}

func (h *DebugHandler) HandleRFID(rfid string) {
	log.Print("Received RFID: ", rfid)
	h.m[h.lineNo] += rfid
	h.t.WriteLCD(h.lineNo, rfid)
}

func (h *DebugHandler) HandleTick() {
	log.Print("Received tick")
}

//-----------------------
type AccessHandler struct {
	currentCode      string
	lastKeypressTime time.Time
	auth             *Authenticator
	t                *TerminalStub
	currentRFID      string
}

func NewAccessHandler(a *Authenticator) *AccessHandler {
	this := new(AccessHandler)
	this.auth = a
	return this
}

func (h *AccessHandler) Init(t *TerminalStub) {
	h.t = t
	h.initGPIO(7)
	h.initGPIO(8)

}

func (h *AccessHandler) initGPIO(pin int) {
	//Initialize the GPIO stuffs

	//Create pin if it doesn't exist
	f, err := os.OpenFile("/sys/class/gpio/export", os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Creating Pin failed - continuing...", pin, err)
	} else {
		f.Write([]byte(fmt.Sprintf("%d\n", pin)))
		f.Close()
	}

	// Put GPIO in Out mode
	f, err = os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Error! Could not configure GPIO", err)
	}
	f.Write([]byte("out\n"))
	f.Close()

	h.switchRelay(false, pin)

}

func (h *AccessHandler) HandleKeypress(b byte) {

	h.lastKeypressTime = time.Now()
	switch b {
	case '#':
		if h.currentCode != "" {
			h.checkAccess(h.currentCode)
			h.currentCode = ""
		}
	case '*':
		h.currentCode = "" // reset
	default:
		h.currentCode += string(b)
	}
}

func (h *AccessHandler) HandleRFID(rfid string) {
	//Split the RFID
	rfid = strings.TrimSpace(strings.Split(rfid, " ")[1])
	//Crude debounce
	if h.currentRFID == rfid {
		log.Println("debounce")
		return
	}
	h.currentRFID = rfid
	h.checkAccess(rfid)
}

func (h *AccessHandler) Open(t Target) {
	if t == TARGET_DOWNSTAIRS {
		h.switchRelay(true, 7)
		time.Sleep(2 * time.Second)
		h.switchRelay(false, 7)
	}
}

func (h *AccessHandler) HandleTick() {
	h.currentRFID = ""

	kKeypadTimeout := 30 * time.Second
	if time.Now().Sub(h.lastKeypressTime) > kKeypadTimeout && h.currentCode != "" {
		//indicate timeout
		h.currentCode = ""
		h.t.BuzzSpeaker("L", 500)
	}
}

func (h *AccessHandler) switchRelay(switch_on bool, pin int) {
	// TODO(hzeller)
	// Hacky for now, this needs to be handled somewhere else. We always
	// use pin 7 for now.

	if pin != 7 && pin != 8 {
		log.Fatal("You suck - pin 7 or 8")
	}

	gpioFile := fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin)
	f, err := os.OpenFile(gpioFile, os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Error! Could not activate relay", err)
		return
	}
	if switch_on {
		f.Write([]byte("0\n")) // negative logic.
	} else {
		f.Write([]byte("1\n"))
	}
	f.Close()
}

func (h *AccessHandler) checkAccess(code string) {
	target := Target(h.t.GetTerminalName())
	if h.auth.AuthUser(code, target) {
		log.Print("Open gate.")
		h.t.ShowColor("G")
		h.t.BuzzSpeaker("H", 500)
		h.Open(target)
		h.t.ShowColor("")
	} else {
		log.Print("Invalid code ", code)
		h.t.ShowColor("R")
		h.t.BuzzSpeaker("L", 200)
		time.Sleep(500)
		h.t.ShowColor("")
	}
}
