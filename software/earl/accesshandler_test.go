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
	allow map[ACKey]bool
}

func NewMockAuthenticator() *MockAuthenticator {
	return &MockAuthenticator{
		allow: make(map[ACKey]bool)}
}

func (a *MockAuthenticator) AuthUser(code string, target Target) (bool, string) {
	_, ok := a.allow[ACKey{code, target}]
	if ok {
		return true, ""
	} else {
		return false, "MockAuthenticator says: user doesn't exist"
	}
}
func (a *MockAuthenticator) AddNewUser(authentication_user string, user User) (bool, string) {
	return false, ""
}
func (a *MockAuthenticator) FindUser(code string) *User { return nil }

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
	t      *testing.T
	opened map[Target]bool
}

func NewMockDoorActions(t *testing.T) *MockDoorActions {
	return &MockDoorActions{
		t:      t,
		opened: make(map[Target]bool)}
}

func (doorActions *MockDoorActions) OpenDoor(target Target) {
	doorActions.opened[target] = true
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

func PressKeys(h *AccessHandler, keys string) {
	for _, key := range keys {
		h.HandleKeypress(byte(key))
	}
}

func TestValidAccessCode(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"123456", Target("mock")}] = true
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(auth, doorActions)
	handler.Init(term)
	handler.clock = MockClock{}
	PressKeys(handler, "123456#")
	term.expectColor("G")
	term.expectBuzz(Buzz{"H", 500})
	doorActions.expectOpenState(true, Target("mock"))
}

func TestInvalidAccessCode(t *testing.T) {
	term := NewMockTerminal(t)
	auth := NewMockAuthenticator()
	auth.allow[ACKey{"123456", Target("mock")}] = true
	doorActions := NewMockDoorActions(t)
	handler := NewAccessHandler(auth, doorActions)
	handler.Init(term)
	handler.clock = MockClock{}
	PressKeys(handler, "654321#")
	term.expectColor("R")
	term.expectBuzz(Buzz{"L", 200})
	doorActions.expectOpenState(false, Target("mock"))
	if doorActions.openedAnyDoor() {
		t.Errorf("There are doors opened, but shouldn't")
	}
}

// test ideas:
//  - wrong code: buzz low and red light
//  - too short code: don't buzz
