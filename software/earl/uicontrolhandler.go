package main

// TODO
//  - A single member can give a day's pass, 2 members a user
//  - How to enter names for members ? For initiall mass-adding: on console
import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type UIState int

const (
	StateIdle               = UIState(0) // When there is nothing to do; idle screen.
	StateDisplayInfoMessage = UIState(1) // Interrupt idle screen and show info message
	StateWaitMemberCommand  = UIState(2) // Member showed RFID; awaiting instruction
	StateAddAwaitNewRFID    = UIState(3) // Member adds new user: wait for new user RFID
)

type UIControlHandler struct {
	// Backend services
	auth Authenticator

	t Terminal

	// Entering name on the console allows keyboard interaction when
	// a new user is added to provide the name. This is mostly needed
	// in our initial phase adding new users
	enterNameConsole bool // Request name on console ?

	authUserCode string // current active member code

	state        UIState   // state of our state machine
	stateTimeout time.Time // timeout of current state
}

func NewControlHandler(authenticator Authenticator, nameOnConsole bool) *UIControlHandler {
	return &UIControlHandler{
		auth:             authenticator,
		enterNameConsole: nameOnConsole,
	}
}

func (u *UIControlHandler) setState(state UIState, timeout_in time.Duration) {
	u.state = state
	u.stateTimeout = time.Now().Add(timeout_in)
}

func (u *UIControlHandler) backToIdle() {
	u.state = StateIdle
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
	if u.state == StateWaitMemberCommand && key == '1' {
		u.t.WriteLCD(0, "Read new user RFID")
		u.t.WriteLCD(1, "[*] Cancel")
		u.setState(StateAddAwaitNewRFID, 30*time.Second)
		return
	}
}

func (u *UIControlHandler) HandleRFID(rfid string) {
	// The ID comes as "<length> <code>". Get the code.
	rfid = strings.TrimSpace(strings.Split(rfid, " ")[1])

	switch u.state {
	case StateIdle:
		user := u.auth.FindUser(rfid)
		if user == nil {
			u.t.WriteLCD(0, "      Unknown RFID")
			u.t.WriteLCD(1, "Ask a member to register")
		} else {
			switch user.UserLevel {
			case LevelMember:
				u.authUserCode = rfid
				u.t.WriteLCD(0, fmt.Sprintf("Howdy %s", user.Name))
				u.t.WriteLCD(1, "[*] Cancel  [1] Add User")
				u.setState(StateWaitMemberCommand, 5*time.Second)

			case LevelUser:
				if user.HasContactInfo() {
					u.t.WriteLCD(0, "Hi "+user.Name)
				} else {
					exp := user.ExpiryDate(time.Now())
					u.t.WriteLCD(0, fmt.Sprintf("Up to %s",
						exp.Format("Up to 2006-01-02 15:04")))
				}
				if user.InValidityPeriod(time.Now()) {
					u.t.WriteLCD(1, "This RFID opens doors :)")
				} else {
					u.t.WriteLCD(1, "Ask member to renew")
				}
				u.setState(StateDisplayInfoMessage, 2*time.Second)

			case LevelLegacy:
				// This should never happen. Display anyway.
				u.t.WriteLCD(1, "Valid RFID to open Gate")
				u.setState(StateDisplayInfoMessage, 2*time.Second)
			}
		}

	case StateAddAwaitNewRFID:
		userName := "<via-lcd>"
		// TODO: this manual input is likely not needed in the future.
		// The members should have a name in the file (rest can be
		// anonymous). So 'upgrading' someone in the future is probably
		// easier by directly editing the CSV
		if u.enterNameConsole {
			u.t.WriteLCD(0, "Enter name on console")
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("Name for %-8s: ", rfid)
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if len(text) > 0 {
				userName = text
				log.Printf("Got name from console '%s'", text)
			}
		}
		newUser := User{
			Name:      userName,
			UserLevel: LevelUser}
		newUser.SetAuthCode(rfid)
		if ok, msg := u.auth.AddNewUser(u.authUserCode, newUser); ok {
			u.t.WriteLCD(0, "Success! User added.")
		} else {
			u.t.WriteLCD(0, msg)
		}
		u.t.WriteLCD(1, "[*] Done    [1] Add More")
		u.setState(StateWaitMemberCommand, 5*time.Second)
	}
}

func (u *UIControlHandler) HandleTick() {
	if u.state != StateIdle && time.Now().After(u.stateTimeout) {
		u.backToIdle()
	}
	if u.state == StateIdle {
		u.displayIdleScreen()
	}
}
