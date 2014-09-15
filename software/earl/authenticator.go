package main

import (
	"bufio"
	"io"
	"os"
	"time"
	"log"
	"regexp"
)

type User struct {
	Name string
	Level string
}


type Authenticator struct {
	codeFilename string
	legacyCodeFilename string
	lastChange   time.Time // last file timestamp we know; reload if file is newer
	validUsers   map[string]User
	legacyCodes	 map[string]bool
}

func NewAuthenticator(codeFilename string, legacyCodeFilename string) *Authenticator {
	a := new(Authenticator)
	a.codeFilename = codeFilename
	a.legacyCodeFilename = legacyCodeFilename


	a.validUsers = make(map[string]User)
	a.legacyCodes = make(map[string]bool)
	a.readLegacyFile()
	return a
}

func (a *Authenticator) readLegacyFile() {
	f, err := os.Open(a.legacyCodeFilename)
	if err != nil {
		log.Fatal("Could not read key file", err)
	}
	reader := bufio.NewReader(f)

	scanregex := regexp.MustCompile("^([0-9]+)")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Fatal("Could not read file", err)
			}
			break
		}
		matches := scanregex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		code := matches[1]
		log.Printf("Loaded legacy code %q\n", code)
		a.legacyCodes[code] = true
	}

}

// Check if RFID access is granted. Initially a boolean, but
// might be later a flag-set for different access levels
func (a *Authenticator) RFIDAccessGranted(id string) bool {
	return false
}
