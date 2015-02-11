// Exchange of application level Events.
//
// Some situations cannot be handled locally by terminal event handlers, but
// have to be dealt with somewhere else (e.g. Doorbell button pressed somewhere
// should all be handled by the LCD control terminal).
//
// The appliation bus allows interested parties to register for events
// which they get handed out to everyone.
package main

type AppEventType int

const (
	// Events
	AppDoorbellPressedEvent = iota // Someone rang the doorbell
	AppDoorSensorEvent             // Door opened/closed

	// Actions
	AppOpenRequest // Request to open door
)

// We keep it simple - an event is identified by an enumeration
type AppEvent struct {
	ev     AppEventType
	target Target
	source string // Mostly FYI: what subsystem generated it
	msg    string // FYI, good to display to a human user
	value  int    // context specific value
}

type AppEventChannel chan *AppEvent
type ApplicationBus struct {
	receivers map[AppEventChannel]bool
	sync_ops  chan func()
}

func NewApplicationBus() *ApplicationBus {
	bus := &ApplicationBus{
		receivers: make(map[AppEventChannel]bool),
		sync_ops:  make(chan func(), 1),
	}
	go bus.run()
	return bus
}

func (b *ApplicationBus) Post(event *AppEvent) {
	b.sync_ops <- func() {
		for channel, _ := range b.receivers {
			channel <- event
		}
	}
}

func (b *ApplicationBus) Subscribe(channel AppEventChannel) {
	b.sync_ops <- func() { b.receivers[channel] = true }
}

func (b *ApplicationBus) Unsubscribe(channel AppEventChannel) {
	b.sync_ops <- func() { delete(b.receivers, channel) }
}

func (b *ApplicationBus) run() {
	for {
		op := <-b.sync_ops
		op()
	}
}
