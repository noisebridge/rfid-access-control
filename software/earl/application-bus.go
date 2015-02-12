// Central hub to exchange application level Events.
//
// Many situations cannot be handled locally by terminal event handlers, but
// have to be delegated somewhere else (e.g. Doorbell button pressed
// somewhere should all be handled by the LCD control terminal and also by the
// physical doorbell tone-generator). Or the reverse: opening a door due to
// some separate event should be reflected in the terminals' green light as well.
//
// Also it is possible to trigger these events artificially, e.g. through
// some web-service.
//
// Since each of these handlers are possibly running in different threads, we
// separate thread-boundaries with channels and have each handler deal with
// things in their own pace.
//
// The appliation bus allows interested parties to Subscribe() to events that
// are being Post()ed to the bus.
package main

import (
	"time"
)

type AppEventType int

const (
	// Events
	AppDoorbellTriggerEvent     = iota // Doorbell triggered for target
	AppDoorSensorEvent                 // Target door opened/closed
	AppOpenRequest                     // Request to open door for target.
	AppSnoozeBellRequest               // Request to snooze bell until given timeout
	applicationBusInternalFlush        // used internally. Never delivered
)

// We keep it simple and somewhat un-typed: an event is identified by an
// enumeration, and optional parameters are passed alongside.
type AppEvent struct {
	// Required parameters
	ev     AppEventType
	target Target // The target for which this event is meant.
	source string // Mostly FYI: what subsystem generated it
	msg    string // FYI, good to display to a human user

	// Optional paramters, depending on context.
	timeout time.Time
	value   int
}

type AppEventChannel chan *AppEvent
type ApplicationBus struct {
	receivers        map[AppEventChannel]bool
	syncedOperations chan func()
	isRunning        bool
}

func NewApplicationBus() *ApplicationBus {
	bus := &ApplicationBus{
		receivers:        make(map[AppEventChannel]bool),
		syncedOperations: make(chan func(), 1),
		isRunning:        true,
	}
	go bus.run()
	return bus
}

func (b *ApplicationBus) Post(event *AppEvent) {
	b.syncedOperations <- func() {
		for channel, _ := range b.receivers {
			if event.ev != applicationBusInternalFlush {
				channel <- event
			}
		}
	}
}

func (b *ApplicationBus) Flush() {
	// Since we have a syncedOperations channel of length 1, this will
	// make sure that we wait until the previous operation is through.
	b.Post(&AppEvent{ev: applicationBusInternalFlush})
}

func (b *ApplicationBus) Subscribe(channel AppEventChannel) {
	b.syncedOperations <- func() { b.receivers[channel] = true }
}

func (b *ApplicationBus) Unsubscribe(channel AppEventChannel) {
	b.syncedOperations <- func() { delete(b.receivers, channel) }
}

func (b *ApplicationBus) Shutdown() {
	b.syncedOperations <- func() { b.isRunning = false }
}

func (b *ApplicationBus) run() {
	for b.isRunning {
		op := <-b.syncedOperations
		op()
	}
}
