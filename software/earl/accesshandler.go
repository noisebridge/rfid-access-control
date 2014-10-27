package main

import (
	"log"
	"strings"
	"time"
)

type AccessHandler struct {
	currentCode      string
	lastKeypressTime time.Time
	auth             Authenticator
	t                Terminal
	doorActions      DoorActions
	currentRFID      string
	clock            Clock
}

func NewAccessHandler(a Authenticator, actions DoorActions) *AccessHandler {
	this := &AccessHandler{
		auth:        a,
		doorActions: actions,
		clock:       RealClock{}}
	return this
}

func (h *AccessHandler) Init(t Terminal) {
	h.t = t
}

func (h *AccessHandler) HandleKeypress(b byte) {
	h.lastKeypressTime = h.clock.Now()
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

func (h *AccessHandler) HandleTick() {
	h.currentRFID = ""

	kKeypadTimeout := 30 * time.Second
	if h.clock.Now().Sub(h.lastKeypressTime) > kKeypadTimeout &&
		h.currentCode != "" {
		// indicate timeout
		h.currentCode = ""
		h.t.BuzzSpeaker("L", 500)
	}
}

func (h *AccessHandler) checkAccess(code string) {
	target := Target(h.t.GetTerminalName())
	if h.auth.AuthUser(code, target) {
		log.Print("Open gate.")
		h.t.ShowColor("G")
		h.t.BuzzSpeaker("H", 500)
		h.doorActions.OpenDoor(target)
		h.t.ShowColor("")
	} else {
		log.Print("Invalid code ", code)
		h.t.ShowColor("R")
		h.t.BuzzSpeaker("L", 200)
		time.Sleep(500)
		h.t.ShowColor("")
	}
}
