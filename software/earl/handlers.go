package main

import (
	"log"
	"time"
	"os"
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
	currentCode string
	timeout int
	auth *Authenticator
	t *TerminalStub
}

func NewAccessHandler(a *Authenticator) *AccessHandler {
	this := new(AccessHandler)
	this.auth = a
	return this
}

func (h *AccessHandler) Init(t *TerminalStub) {
	h.t = t
}

func (h *AccessHandler) HandleKeypress(b byte) {
	switch b {
	case '#':
		h.checkPinAccess()
	case '*':
		h.currentCode = ""   // reset
	default:
		h.currentCode += string(b)
	}
}

func (h *AccessHandler) HandleRFID(rfid string) {
}

func (h *AccessHandler) HandleTick() {
	if h.timeout > 0 {
		h.timeout -= 1
	}
}

func (h *AccessHandler) switchRelay(switch_on bool) {
	// TODO(hzeller)
	// Hacky for now, this needs to be handled somewhere else. We always
	// use pin 7 for now.
	f, err := os.OpenFile("/sys/class/gpio/gpio7/value", os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Error while reading user file", err)
		return
	}
	if switch_on {
		f.Write([]byte("0\n"))  // negative logic.
	} else {
		f.Write([]byte("1\n"))
	}
	f.Close()
}

func (h *AccessHandler) checkPinAccess() {
	log.Print("Got pin code")
	if h.auth.LegacyKeycodeAccessGranted(h.currentCode) {
		log.Print("Open gate.")
		h.switchRelay(true);
		time.Sleep(2 * time.Second)
		h.switchRelay(false);
	} else {
		log.Print("Invalid code.");
	}
	h.currentCode = ""
}
