package main

import (
	"log"
	"strings"
	"time"
)

type AccessHandler struct {
	// Backend services
	auth        Authenticator
	doorActions DoorActions
	clock       Clock

	t Terminal // Our terminal we can do operations on

	// Current state
	currentCode      string    // PIN typed so far on keypad
	lastKeypressTime time.Time // Last touch of key to reset
	currentRFID      string    // Current RFID we received
	lastCurrentRFID  time.Time // Time we have seen the current RFID
}

const (
	kRFIDRepeatDebounce = 2 * time.Second  // RFID is repeated. Pace down.
	kKeypadTimeout      = 30 * time.Second // Timeout keypad user gone.
	kMinimumCodeLen     = 4                // Mostly for PINs; RFID are > 8
)

func NewAccessHandler(a Authenticator, actions DoorActions) *AccessHandler {
	return &AccessHandler{
		auth:        a,
		doorActions: actions,
		clock:       RealClock{}}
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
	// Don't bother with too short codes. In particular, don't buzz
	// or flash lights to not to seem overly interactive.
	if len(code) < kMinimumCodeLen {
		return
	}
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
