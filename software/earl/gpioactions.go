package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

type GPIOActions struct {
}

func (g *GPIOActions) Init() {
	g.initGPIO(7)
	g.initGPIO(8)
}

func (h *GPIOActions) OpenDoor(which Target) {
	switch which {
	case TargetDownstairs:
		h.switchRelay(true, 7)
		time.Sleep(2 * time.Second)
		h.switchRelay(false, 7)

	default:
		log.Printf("Dude, don't know how to open '%s'", which)
	}
}

func (g *GPIOActions) initGPIO(pin int) {
	//Initialize the GPIO stuffs

	//Create pin if it doesn't exist
	f, err := os.OpenFile("/sys/class/gpio/export", os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Creating Pin failed - continuing...", pin, err)
	} else {
		f.Write([]byte(fmt.Sprintf("%d\n", pin)))
		f.Close()
	}

	// Put GPIO in Out mode
	f, err = os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Error! Could not configure GPIO", err)
	}
	f.Write([]byte("out\n"))
	f.Close()

	g.switchRelay(false, pin)
}

func (g *GPIOActions) switchRelay(switch_on bool, gpio_pin int) {
	// TODO(hzeller)
	// Hacky for now, this needs to be handled somewhere else. We always
	// use gpio_pin 7 for now.

	if gpio_pin != 7 && gpio_pin != 8 {
		log.Fatal("You suck - gpio_pin 7 or 8")
	}

	gpioFile := fmt.Sprintf("/sys/class/gpio/gpio%d/value", gpio_pin)
	f, err := os.OpenFile(gpioFile, os.O_WRONLY, 0444)
	if err != nil {
		log.Print("Error! Could not activate relay", err)
		return
	}
	if switch_on {
		f.Write([]byte("0\n")) // negative logic.
	} else {
		f.Write([]byte("1\n"))
	}
	f.Close()
}
