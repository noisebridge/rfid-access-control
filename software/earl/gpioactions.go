// These are actions wired to GPIO ports of the Raspberry Pi.
// The EventLoop listens for incoming requests on the ApplicationBus, but
// also can send events to the ApplicationBus from input-GPIO pins, e.g.
// reed contacts or independent doorbell buttons.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	WavPlayer = "/usr/bin/aplay"

	// Length of time to open the door and minimum time between.
	defaultDoorOpenTime      = 2 * time.Second
	defaultDoorOpenRateLimit = 500 * time.Millisecond

	// Don't allow to ring more often than this.
	defaultDoorbellRatelimit = 15 * time.Second
)

type GPIOActions struct {
	doorbellDirectory   string
	nextAllowedOpenTime map[Target]time.Time
	nextAllowedRingTime map[Target]time.Time
}

// Create this, then call EventLoop() to hook into system.
func NewGPIOActions(wavDir string) *GPIOActions {
	result := &GPIOActions{
		doorbellDirectory:   wavDir,
		nextAllowedOpenTime: make(map[Target]time.Time),
		nextAllowedRingTime: make(map[Target]time.Time),
	}
	result.initGPIO(7)
	result.initGPIO(8)
	result.initGPIO(9)
	result.initGPIO(11)
	return result
}

// Receive events from the bus and act on it.
// (later: if we read reed contacts, send AppDoorSensorEvents)
func (g *GPIOActions) EventLoop(bus *ApplicationBus) {
	appEvents := make(AppEventChannel, 2)
	bus.Subscribe(appEvents)
	for {
		event := <-appEvents
		switch event.Ev {
		case AppOpenRequest:
			g.openDoor(event.Target)
		case AppDoorbellTriggerEvent:
			g.ringBell(event.Target)
		case AppHushBellRequest:
			g.nextAllowedRingTime[event.Target] = event.Timeout
		}
	}
}

func (g *GPIOActions) openDoor(which Target) {
	if time.Now().Before(g.nextAllowedOpenTime[which]) {
		// We don't want to interfere with ourself currently opening.
		return
	}
	g.nextAllowedOpenTime[which] = time.Now().Add(defaultDoorOpenTime + defaultDoorOpenRateLimit)

	gpio_pin := -1
	switch which {
	case TargetDownstairs:
		gpio_pin = 7

	case TargetUpstairs:
		gpio_pin = 11

	case TargetElevator:
		gpio_pin = 9

	default:
		log.Printf("DoorAction: Don't know how to open '%s'", which)
	}
	// Maybe when we see a door-open event for this target, fall back
	// to non-buzzing immediately after ?
	if gpio_pin > 0 {
		go func() {
			g.switchRelay(true, gpio_pin)
			time.Sleep(defaultDoorOpenTime)
			g.switchRelay(false, gpio_pin)
		}()
	}

	// The door was opened, so allow the doorbell to ring again right away.
	g.nextAllowedRingTime[which] = time.Now()
}

func (g *GPIOActions) ringBell(which Target) {
	if time.Now().Before(g.nextAllowedRingTime[which]) {
		return // Hushed.
	}
	filename := g.doorbellDirectory + "/" + string(which) + ".wav"
	_, err := os.Stat(filename)
	msg := ""
	if err == nil {
		// Inform pegasus about doorbell, so that it can ring. But
		// time-out so that network issues don't cause thread-eating.
		// (there is a delay currently in the network set-up. Disable
		// for now)
		//go exec.Command("/usr/bin/curl", "-q", "-m", "3", "http://pegasus.noise/bell/?tone="+string(which)).Run()
		go exec.Command(WavPlayer, filename).Run()
	} else {
		msg = ": [ugh, file not found!]"
	}
	log.Printf("Ringing doorbell for %s (%s%s)", which, filename, msg)
	g.nextAllowedRingTime[which] = time.Now().Add(defaultDoorbellRatelimit)
}

func (g *GPIOActions) initGPIO(gpio_pin int) {
	// Initialize the GPIO stuffs
	// Create gpio_pin if it doesn't exist
	f, err := os.OpenFile("/sys/class/gpio/export", os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Creating GPIO-pin failed - continuing...", gpio_pin, err)
	} else {
		f.Write([]byte(fmt.Sprintf("%d\n", gpio_pin)))
		f.Close()
	}

	// Put GPIO in Out mode
	f, err = os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", gpio_pin), os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Error! Could not configure GPIO", err)
	}
	f.Write([]byte("out\n"))
	f.Close()

	g.switchRelay(false, gpio_pin) // initial state.
}

func (g *GPIOActions) switchRelay(switch_on bool, gpio_pin int) {
	if gpio_pin != 7 && gpio_pin != 8 && gpio_pin != 9 && gpio_pin != 11 {
		log.Print("GPIO needs to be one of 7,8,9,11!")
	}

	gpioFile := fmt.Sprintf("/sys/class/gpio/gpio%d/value", gpio_pin)
	f, err := os.OpenFile(gpioFile, os.O_WRONLY, 0444)
	if err != nil {
		log.Printf("Error! Could not activate relay (%t): %s", switch_on, err)
		return
	}
	if switch_on {
		f.Write([]byte("0\n")) // negative logic.
	} else {
		f.Write([]byte("1\n"))
	}
	f.Close()
}
