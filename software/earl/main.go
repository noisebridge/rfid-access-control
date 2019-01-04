package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

// Inserted later w/ a linker flag.
// Check the Makefile for details.
var Branch string
var BuildDate string
var Revision string
var Version string

// Prometheus namespace
var metricNamespace = "earl"

// Each access point has their own name. The terminals can identify
// by that name.

type Target string // TODO: find better name for this type
func (s Target) String() string {
	return string(s)
}

const (
	TargetDownstairs = Target("gate")
	TargetUpstairs   = Target("upstairs")
	TargetElevator   = Target("elevator")
	TargetControlUI  = Target("control") // UI to add new users.
)

var (
	targets = []Target{
		TargetDownstairs,
		TargetUpstairs,
		TargetElevator,
		TargetControlUI,
	}
)

const (
	maxLCDRows                  = 2
	maxLCDCols                  = 24
	defaultBaudrate             = 9600
	initialReconnectOnErrorTime = 2 * time.Second
	maxReconnectOnErrorTime     = 60 * time.Second
	idleTickTime                = 500 * time.Millisecond
)

func parseArg(arg string) (devicepath string, baudrate int) {
	split := strings.Split(arg, ":")
	devicepath = split[0]
	baudrate = defaultBaudrate
	if len(split) > 1 {
		var err error
		if baudrate, err = strconv.Atoi(split[1]); err != nil {
			panic(err)
		}
	}
	return
}

type Backends struct {
	authenticator Authenticator
	appEventBus   *ApplicationBus
}

func printVersionInfo() {
	fmt.Printf("Version: %s\n", Version)
}

func printUserList(auth *FileBasedAuthenticator) {
	longest_name := 1
	longest_contact := 1
	auth.IterateUsers(func(user User) {
		if len(user.Name) > longest_name {
			longest_name = len(user.Name)
		}
		if len(user.ContactInfo) > longest_contact {
			longest_contact = len(user.ContactInfo)
		}
	})

	auth.IterateUsers(func(user User) {
		fmt.Printf("%*s %*s %-14s ",
			-longest_name, user.Name,
			-longest_contact, user.ContactInfo, user.UserLevel)
		timeFrom, timeTo := user.AccessHours()
		fmt.Printf("\u231a %02d:00..%02d:00 ", timeFrom, timeTo)

		exp := user.ExpiryDate(time.Now())
		validityPeriod := user.InValidityPeriod(time.Now())
		if !exp.IsZero() {
			if !validityPeriod {
				fmt.Printf("\033[1;31mExpired ")
			} else {
				fmt.Printf("\033[1;32mExpires ")
			}
			fmt.Print(exp.Format("2006-01-02 15:04"))
			fmt.Printf("\033[0m")
		}
		fmt.Println()
	})
}

func handleSerialDevice(devicepath string, baud int, backends *Backends) {
	var t *SerialTerminal
	connect_successful := true
	retry_time := initialReconnectOnErrorTime
	for {
		if !connect_successful {
			time.Sleep(retry_time)
			retry_time *= 2 // exponential backoff.
			if retry_time > maxReconnectOnErrorTime {
				retry_time = maxReconnectOnErrorTime
			}
		}

		connect_successful = false

		t, _ = NewSerialTerminal(devicepath, baud)
		if t == nil {
			continue
		}

		// Terminals are dispatched by name. There are different handlers
		// for the name e.g. handlers that deal with reading codes
		// and opening doors, but also the UI handler dealing with
		// adding new users.
		var handler TerminalEventHandler
		switch Target(t.GetTerminalName()) {
		case TargetDownstairs, TargetUpstairs, TargetElevator:
			handler = NewAccessHandler(backends)

		case TargetControlUI:
			handler = NewControlHandler(backends)

		default:
			log.Printf("%s:%d: Terminal with unrecognized name '%s'",
				devicepath, baud, t.GetTerminalName())
		}

		if handler != nil {
			connect_successful = true
			retry_time = initialReconnectOnErrorTime
			log.Printf("%s:%d: connected to '%s'",
				devicepath, baud, t.GetTerminalName())
			backends.appEventBus.Post(&AppEvent{
				Ev:     AppTerminalConnect,
				Target: Target(t.GetTerminalName()),
				Msg:    fmt.Sprintf("%s:%d", devicepath, baud),
				Source: "serialdevice",
			})
			t.RunEventLoop(handler, backends.appEventBus)
			backends.appEventBus.Post(&AppEvent{
				Ev:     AppTerminalDisconnect,
				Target: Target(t.GetTerminalName()),
				Msg:    fmt.Sprintf("%s:%d", devicepath, baud),
				Source: "serialdevice",
			})
		}
		t.shutdown()
		t = nil
	}
}

func init() {
	version.Branch = Branch
	version.BuildDate = BuildDate
	version.Revision = Revision
	version.Version = Version
	prometheus.MustRegister(version.NewCollector("earl"))
}

func main() {
	userFileName := flag.String("users", "", "User Authentication file.")
	logFileName := flag.String("logfile", "", "The log file, default = stdout")
	doorbellDir := flag.String("belldir", "", "Directory that contains upstairs.wav, gate.wav etc. Wav needs to be named like")
	httpPort := flag.Int("httpport", -1, "Port to listen HTTP requests on")
	tcpPort := flag.Int("tcpport", -1, "Port to listen for TCP requests on")
	list_users := flag.Bool("list-users", false, "List users and exit")
	show_version := flag.Bool("version", false, "Print version info")

	flag.Parse()

	if *show_version {
		printVersionInfo()
		return
	}

	if *logFileName != "" {
		logfile, err := os.OpenFile(*logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Error opening log file", err)
		}
		defer logfile.Close()
		log.SetOutput(logfile)
	}

	log.Printf("Starting... version: %s\n", Version)

	if len(flag.Args()) < 1 && !*list_users {
		fmt.Fprintf(os.Stderr,
			"Expected list of serial ports."+
				"usage: %s [options] <serial-device>[:baudrate] [<serial-device>[:baudrate]...]\nOptions\n",
			os.Args[0])
		flag.PrintDefaults()
		return
	}

	appEventBus := NewApplicationBus()
	authenticator := NewFileBasedAuthenticator(*userFileName,
		appEventBus)
	backends := &Backends{
		authenticator: authenticator,
		appEventBus:   appEventBus,
	}

	if authenticator == nil {
		log.Fatal("Can't continue without authenticator.")
	}

	// If we just requested to list users, do this and exit.
	if *list_users {
		printUserList(authenticator)
		return
	}

	actions := NewGPIOActions(*doorbellDir)
	go actions.EventLoop(appEventBus)

	// For each serial interface, we run an indepenent loop
	// making sure we are constantly connected.
	for _, arg := range flag.Args() {
		devicepath, baudrate := parseArg(arg)
		go handleSerialDevice(devicepath, baudrate, backends)
	}

	if *httpPort > 0 && *httpPort <= 65535 {
		mux := http.NewServeMux()
		server := &http.Server{
			Addr: fmt.Sprintf(":%d", *httpPort),
			// JSON events listeners should be kept open for a while
			WriteTimeout: 3600 * time.Second,
			Handler:      mux,
		}
		mux.Handle("/metrics", promhttp.Handler())
		NewApiServer(appEventBus, mux)
		go server.ListenAndServe()
	}

	if *tcpPort > 0 && *tcpPort <= 65535 {
		tcpServer := NewTcpServer(appEventBus, *tcpPort)
		go tcpServer.Run()
	}

	log.Println("Ready.")
	backends.appEventBus.Post(&AppEvent{
		Ev:     AppEarlStarted,
		Msg:    "Earl version " + Version + " started. Ready to serve.",
		Source: "main",
	})

	var block_forever chan bool
	<-block_forever
}
