// AccessHandler.
//
// A TerminalEventHandler that deals with the terminal mounted to an entrance,
// receiving PIN and RFID events and triggering opening gates and doors (through
// an implementation of interface DoorActions) if appropriate.
// Also user-feedback with LEDs and feedback tones.
// Each entrance has its own independent instance running.
package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
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
	kKeypadTimeout      = 30 * time.Second // Timeout: user stopped typing
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
			h.checkAccess("keypad", h.currentCode)
			h.currentCode = ""
		} else {
			// Consider this a doorbell ?
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
		return // debounce.
	}
	h.currentRFID = rfid
	h.lastCurrentRFID = h.clock.Now()
	h.checkAccess("RFID", rfid)
}

func (h *AccessHandler) HandleTick() {
	if h.clock.Now().Sub(h.lastCurrentRFID) > kRFIDRepeatDebounce {
		h.currentRFID = ""
	}

	// Keypad got a partial code, but never finished with '#'
	if h.clock.Now().Sub(h.lastKeypressTime) > kKeypadTimeout &&
		h.currentCode != "" {
		h.currentCode = ""
		h.t.BuzzSpeaker("L", 500) // indicate timeout
	}
}

// Hashing a value in a way that we can't recover the content of the value,
// but only can compare if we get the same value.
func scrubLogValue(in string) string {
	hashgen := md5.New()
	io.WriteString(hashgen, in)
	return hex.EncodeToString(hashgen.Sum(nil))[0:6]
}

func (h *AccessHandler) checkAccess(origin string, code string) {
	// Don't bother with too short codes. In particular, don't buzz
	// or flash lights to not to seem overly interactive.
	if !hasMinimalCodeRequirements(code) {
		return
	}
	target := Target(h.t.GetTerminalName())
	user := h.auth.FindUser(code)
	auth_result, msg := h.auth.AuthUser(code, target)
	if user != nil && auth_result == AuthOk {
		h.t.ShowColor("G")
		h.t.BuzzSpeaker("H", 500)
		// Be sparse, don't log user, but keep track of level.
		log.Printf("%s: opened. %s Type=%s",
			target, origin, user.UserLevel)
		h.doorActions.OpenDoor(target)
		h.t.ShowColor("")
	} else {
		// This is either an invalid RFID (or used outside the
		// validity), or a PIN-code, which is not valid for user
		// access.
		// In that case, we log a scrubbed code - we won't be able
		// to recover the code (we don't store the plain code anywhere
		// to create a reverse table), but can see patterns when the
		// same thing happens multiple times.
		log.Printf("%s: denied. %s | %s (%s)",
			target, msg, origin, scrubLogValue(code))
		if auth_result == AuthFail {
			h.t.ShowColor("R")
		} else {
			// Show blue for authentication that is just failing
			// due to expired token/outside daytime. That gives some
			// valueable feedback to the user.
			h.t.ShowColor("B")
			// TODO: should this trigger a doorbell ? Usually if
			// someone is there, they might open the door.
		}
		h.t.BuzzSpeaker("L", 200)
		time.Sleep(500 * time.Millisecond)
		h.t.ShowColor("")
	}
}
