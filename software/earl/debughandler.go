package main

import (
	"log"
)

type DebugHandler struct {
	m      [2]string
	lineNo int
	t      Terminal
}

func (h *DebugHandler) Init(t Terminal) {
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
