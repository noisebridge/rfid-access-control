package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// An implementation of the DoorActions interface
type GPIOActions struct {
}

func (g *GPIOActions) Init() {
	g.initGPIO(7)
	g.initGPIO(8)
}

func (g *GPIOActions) OpenDoor(which Target) {
	gpio_pin := -1
	switch which {
	case TargetDownstairs:
		gpio_pin = 7

	case TargetUpstairs:
		gpio_pin = 8

	default:
		log.Printf("Dude, don't know how to open '%s'", which)
	}
	if gpio_pin > 0 {
		log.Printf("Open door '%s'; gpio=%d", which, gpio_pin)
		g.switchRelay(true, gpio_pin)
		time.Sleep(2 * time.Second)
		g.switchRelay(false, gpio_pin)
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
