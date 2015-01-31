package main

import (
	"strings"
	"testing"
	"time"
)

type ACKey struct {
	code   string
	target Target
}

// Implements Athenticator interface.
type MockAuthenticator struct {
	allow map[ACKey]AuthResult
}

func NewMockAuthenticator() *MockAuthenticator {
	return &MockAuthenticator{
		allow: make(map[ACKey]AuthResult)}
}

func (a *MockAuthenticator) AuthUser(code string, target Target) (AuthResult, string) {
	result, ok := a.allow[ACKey{code, target}]
	if !ok {
		return AuthFail, "User does not exist"
	}
	if result == AuthOk {
		return AuthOk, ""
	}
	return result, "MockAuthenticator says: some failure occured"
}

func (a *MockAuthenticator) AddNewUser(authentication_user string, user User) (bool, string) {
	return false, ""
}
func (a *MockAuthenticator) FindUser(code string) *User {
	// Return dummy user as accesshandler likes to independently find it.
	return &User{
		UserLevel: "member",
	}
}
func (a *MockAuthenticator) UpdateUser(auth_code string, user_code string, updater_fun ModifyFun) (bool, string) {
	return false, ""
}

func (a *MockAuthenticator) DeleteUser(auth_code string, user_code string) (bool, string) {
	return false, ""
}

type Buzz struct {
	toneCode string
	duration time.Duration
}

// Implements Terminal interface.
type MockTerminal struct {
	t      *testing.T
	colors string
	buzzes []Buzz
	lcd    [2]string
}

func NewMockTerminal(t *testing.T) *MockTerminal {
	ret := &MockTerminal{t: t}
	return ret
}

func (term *MockTerminal) GetTerminalName() string {
	return "mock"
}

func (term *MockTerminal) ShowColor(colors string) {
	term.colors = term.colors + colors
}

func (term *MockTerminal) BuzzSpeaker(toneCode string, duration time.Duration) {
	term.buzzes = append(term.buzzes, Buzz{toneCode, duration})
}

func (term *MockTerminal) WriteLCD(row int, text string) {
	term.lcd[row] = text
}

func (term *MockTerminal) expectColor(color string) {
	if !strings.Contains(term.colors, color) {
		term.t.Errorf("Expecting color '%v', but seeing colors '%v'", color, term.colors)
	}
}

func (term *MockTerminal) expectBuzz(buzz Buzz) {
	if len(term.buzzes) == 0 {
		term.t.Errorf("Expecting buzz %v but heard nothing", buzz)
		return
	}
	curBuzz := term.buzzes[0]
	term.buzzes = term.buzzes[1:]
	if curBuzz != buzz {
		term.t.Errorf("Expecting buzz %v but heard buzz %v", buzz, curBuzz)
	}
}

type MockDoorActions struct {
	t        *testing.T
	opened   map[Target]bool
	doorbell map[Target]bool
}

func NewMockDoorActions(t *testing.T) *MockDoorActions {
	return &MockDoorActions{
		t:        t,
		opened:   make(map[Target]bool),
		doorbell: make(map[Target]bool),
	}
}

func (doorActions *MockDoorActions) OpenDoor(target Target) {
	doorActions.opened[target] = true
}

func (doorActions *MockDoorActions) RingBell(target Target) {
	doorActions.doorbell[target] = true
}

func (doorActions *MockDoorActions) openedAnyDoor() bool {
	return len(doorActions.opened) != 0
}

func (doorActions *MockDoorActions) expectOpenState(expected bool, target Target) {
	// Both the existence an value in doorActions are to be like expected
	opened, has := doorActions.opened[target]
	if !(has == expected && opened == expected) {
		doorActions.t.Errorf("Expected %v to be open=%v", target, expected)
	}
}

func (doorActions *MockDoorActions) expectDoorbell(expected bool, target Target) {
	// Both the existence an value in doorActions are to be like expected
	opened, has := doorActions.doorbell[target]
	if !(has == expected && opened == expected) {
		doorActions.t.Errorf("Expected %v to be ring=%v", target, expected)
	}
}

func (doorActions *MockDoorActions) resetDoors() {
	doorActions.opened = make(map[Target]bool)
	doorActions.doorbell = make(map[Target]bool)
}

func NewMockBackends(auth Authenticator, actions PhysicalActions) *Backends {
	return &Backends{
		authenticator:   auth,
		physicalActions: actions,
		doorbellUI:      &SimpleDoorbellUI{actions: actions},
	}
}

func PressKeys(h *AccessHandler, keys string) {
	for _, key := range keys {
		h.HandleKeypress(byte(key))
	}
}

func TestValidAccessCode(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"123456", Target("mock")}] = AuthOk
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(NewMockBackends(auth, doorActions))
	handler.Init(term)
	handler.clock = MockClock{}
	PressKeys(handler, "123456#")
	term.expectColor("G")
	term.expectBuzz(Buzz{"H", 500})
	doorActions.expectOpenState(true, Target("mock"))
	doorActions.expectDoorbell(false, Target("mock"))
}

func TestInvalidAccessCode(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"123456", Target("mock")}] = AuthOk
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(NewMockBackends(auth, doorActions))
	handler.Init(term)
	handler.clock = MockClock{}
	PressKeys(handler, "654321#")
	term.expectColor("R")
	term.expectBuzz(Buzz{"L", 200})
	doorActions.expectOpenState(false, Target("mock"))
	doorActions.expectDoorbell(false, Target("mock"))
	if doorActions.openedAnyDoor() {
		t.Errorf("There are doors opened, but shouldn't")
	}
}

func TestExpiredAccessCode(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"123456", Target("mock")}] = AuthExpired
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(NewMockBackends(auth, doorActions))
	handler.Init(term)
	handler.clock = MockClock{}
	PressKeys(handler, "123456#")
	term.expectColor("B") // Blue indicator
	term.expectBuzz(Buzz{"L", 200})
	doorActions.expectOpenState(false, Target("mock"))
	doorActions.expectDoorbell(true, Target("mock"))
	if doorActions.openedAnyDoor() {
		t.Errorf("There are doors opened, but shouldn't")
	}
}

func TestKeypadDoorbell(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(NewMockBackends(auth, doorActions))
	handler.Init(term)
	PressKeys(handler, "#") // Just a single '#' should ring the bell.
	doorActions.expectDoorbell(true, Target("mock"))
	doorActions.expectOpenState(false, Target("mock"))
}

func TestKeypadTimeout(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"123456", Target("mock")}] = AuthOk
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(NewMockBackends(auth, doorActions))
	handler.Init(term)
	mockClock := &MockClock{}
	handler.clock = mockClock

	PressKeys(handler, "123456")                        // missing #
	mockClock.now = mockClock.now.Add(60 * time.Second) // >> keypad timeout
	handler.HandleTick()
	term.expectBuzz(Buzz{"L", 500}) // timeout buzz
	if doorActions.openedAnyDoor() {
		t.Errorf("There are doors opened, but shouldn't")
	}
}

func TestRFIDDebounce(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"rfid-123", Target("mock")}] = AuthOk
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(NewMockBackends(auth, doorActions))
	handler.Init(term)
	mockClock := &MockClock{}
	handler.clock = mockClock

	handler.HandleRFID("8 rfid-123")
	doorActions.expectOpenState(true, Target("mock"))
	doorActions.resetDoors()

	// A quickly coming same RFID should not open the door again
	handler.HandleRFID("8 rfid-123")
	doorActions.expectOpenState(false, Target("mock"))

	// .. but after some de-bounce time, this should work again.
	mockClock.now = mockClock.now.Add(10 * time.Second)
	handler.HandleRFID("8 rfid-123")
	doorActions.expectOpenState(true, Target("mock"))
}

// test ideas:
//  - too short code: don't buzz
