package main

import (
	"fmt"
	"strings"
	"time"
)

type UIState int

const (
	IDLE                 = 0
	DISPLAY_INFO_MESSAGE = 1
	ADD_WAIT_INPUT       = 2
	ADD_AWAIT_NEW_RFID   = 3
)

type UIControlHandler struct {
	// Backend services
	auth Authenticator

	t Terminal

	authUserCode string // current active member code

	state        UIState   // state of our state machine
	stateTimeout time.Time // timeout of current state
}

func NewControlHandler(authenticator Authenticator) *UIControlHandler {
	return &UIControlHandler{
		auth: authenticator,
	}
}

func (u *UIControlHandler) setState(state UIState, timeout_in time.Duration) {
	u.state = state
	u.stateTimeout = time.Now().Add(timeout_in)
}

func (u *UIControlHandler) backToIdle() {
	u.state = IDLE
	u.authUserCode = ""
	u.displayIdleScreen()
}

func (u *UIControlHandler) displayIdleScreen() {
	// TODO: do something fancy every now and then, some animation..
	now := time.Now()
	u.t.WriteLCD(0, "      Noisebridge")
	u.t.WriteLCD(1, now.Format("2006-01-02 [Mon] 15:04"))
}

func (u *UIControlHandler) Init(t Terminal) {
	u.t = t
}

func (u *UIControlHandler) HandleKeypress(key byte) {
	if key == '*' {
		u.backToIdle()
		return
	}
	switch u.state {
	case ADD_WAIT_INPUT:
		if key == '1' {
			u.t.WriteLCD(0, "Read new user RFID")
			u.t.WriteLCD(1, "[*] Cancel")
			u.setState(ADD_AWAIT_NEW_RFID, 30*time.Second)
		}
	}
}

func (u *UIControlHandler) HandleRFID(rfid string) {
	// The ID comes as "<length> <code>". Get the code.
	rfid = strings.TrimSpace(strings.Split(rfid, " ")[1])

	switch u.state {
	case IDLE:
		user := u.auth.FindUser(rfid)
		if user == nil {
			u.t.WriteLCD(0, "      Unknown RFID")
			u.t.WriteLCD(1, "Ask a member to register")
		} else {
			switch user.UserLevel {
			case LevelLegacy:
				// This should never happen. Display anyway.
				u.t.WriteLCD(1, "Valid RFID to open Gate")
				u.setState(DISPLAY_INFO_MESSAGE, 2*time.Second)
			case LevelUser:
				u.t.WriteLCD(1, "Hi there :) Good RFID!")
				u.setState(DISPLAY_INFO_MESSAGE, 2*time.Second)
			case LevelMember:
				u.authUserCode = rfid
				u.t.WriteLCD(0, fmt.Sprintf("Howdy %s",
					user.Name))
				u.t.WriteLCD(1, "[1] Add User [*] Cancel")
				u.setState(ADD_WAIT_INPUT, 5*time.Second)
			}
		}
	case ADD_AWAIT_NEW_RFID:
		new_user := User{
			Name:      "<from-ui>",
			UserLevel: LevelUser,
			Codes:     []string{rfid}}
		if u.auth.AddNewUser(u.authUserCode, new_user) {
			u.t.WriteLCD(0, "Success! User added.")
		} else {
			// TODO: make AddNewUser return error
			u.t.WriteLCD(0, "D'oh - didn't work.")
		}
		u.t.WriteLCD(1, "(1) Add More  (*) Done")
		u.setState(ADD_WAIT_INPUT, 5*time.Second)
	}
}

func (u *UIControlHandler) HandleTick() {
	if u.state != IDLE && time.Now().After(u.stateTimeout) {
		u.state = IDLE
	}
	if u.state == IDLE {
		u.displayIdleScreen()
	}
}
