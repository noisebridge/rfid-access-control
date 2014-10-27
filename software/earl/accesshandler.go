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
	lastCurrentRFID  time.Time
	clock            Clock
}

const (
	kRFIDRepeatDebounce = 2 * time.Second  // RFID is sent while close. Pace down.
	kKeypadTimeout      = 30 * time.Second // Timeout keypad user gone.
)

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
	// The ID comes as "<length> <code>". Get the code.
	rfid = strings.TrimSpace(strings.Split(rfid, " ")[1])

	if h.currentRFID == rfid {
		log.Println("debounce")
		return
	} else {
		h.currentRFID = rfid
		h.lastCurrentRFID = h.clock.Now()
	}
	h.checkAccess(rfid)
}

func (h *AccessHandler) HandleTick() {
	if h.clock.Now().Sub(h.lastCurrentRFID) > kRFIDRepeatDebounce {
		h.currentRFID = ""
	}

	// Keypad got a partial code, but never finished with '#'
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
