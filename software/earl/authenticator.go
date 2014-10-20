package main

import (
	"bufio"
	"io"
	"os"
	"time"
	"log"
	"regexp"
	"encoding/csv"
	//"errors"
)


var local *time.Location 

type Level string

const (
	LEVEL_USER = "user"
	LEVEL_MEMBER = "member"
)


type User struct {
	Name string
	UserLevel Level
	Codes []string
}

type Authenticator struct {
	userFilename string
	legacyCodeFilename string
	lastChange   time.Time // last file timestamp we know; reload if file is newer
	validUsers   map[string]*User
	legacyCodes	 map[string]bool
}

func NewAuthenticator(userFilename string, legacyCodeFilename string) *Authenticator {
	asdf, err := time.LoadLocation("America/Los_Angeles")
	local = asdf
	if err != nil {
		log.Fatal("Time zone death failure bad", err)
	}

	a := new(Authenticator)
	a.userFilename = userFilename
	a.legacyCodeFilename = legacyCodeFilename


	a.validUsers = make(map[string]*User)
	a.legacyCodes = make(map[string]bool)
	a.readLegacyFile()
	a.readUserFile()
	return a
}

func (a *Authenticator) readLegacyFile() {
	if a.legacyCodeFilename == "" {
		log.Println("Legacy key file not provided");
		return
	}
	f, err := os.Open(a.legacyCodeFilename)
	if err != nil {
		log.Fatal("Could not read PIN-key file", err)
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

//
//Read the user CSV file
//
//It is name, level, code[,code...]
func (a *Authenticator) readUserFile() {
	if a.userFilename == "" {
		log.Println("RFID-user file not provided");
		return
	}
	f, err := os.Open(a.userFilename)
	if err != nil {
		log.Fatal("Could not read RFID user-file", err)
	}

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 //variable length fields

	for {
		line, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				log.Fatal("Error while reading user file", err)
			}
			break
		}
		if len(line) < 3 {
			log.Println("Skipping short line", line)
		}
		u := User{Name: line[0], UserLevel: Level(line[1]), Codes: line[2:]}
		log.Println("Got a new user", u)

		for _, code := range u.Codes {
			a.validUsers[code] = &u
		}
	}
}

// Check if RFID access is granted. Initially a boolean, but
// might be later a flag-set for different access levels
func (a *Authenticator) LegacyKeycodeAccessGranted(id string) bool {
	_, ok := a.legacyCodes[id]
	return ok
}

func (a *Authenticator) AuthUser(code string) bool {
	u, ok := a.validUsers[code]
	if !ok {
		return false
	}

	return a.LevelHasAccess(u.UserLevel)
}

func (a *Authenticator) LevelHasAccess(level Level) bool{
	now := time.Now()

	now = now.In(local)
	//Members have access
	if level == LEVEL_MEMBER {
		return true
	} else if level == LEVEL_USER {
		hour, _, _ := now.Clock()
		return hour >= 10 && hour < 22
	}

	return false
}
