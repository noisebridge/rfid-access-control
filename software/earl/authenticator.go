package main

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
	//"errors"
)

var local *time.Location

type Level string

const (
	LEVEL_LEGACY = "legacy"
	LEVEL_USER   = "user"
	LEVEL_MEMBER = "member"
)

type Target string

const (
	TARGET_DOWNSTAIRS = "gate"
	TARGET_UPSTAIRS   = "door"
	// Someday we'll have the network closet locked down
	//TARGET_NETWORK = "closet"
)

type User struct {
	Name      string
	UserLevel Level
	Codes     []string
}

type Authenticator struct {
	userFilename       string
	legacyCodeFilename string
	lastChange         time.Time // last file timestamp we know; reload if file is newer
	validUsers         map[string]*User
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
	a.readLegacyFile()
	a.readUserFile()
	return a
}

func (a *Authenticator) readLegacyFile() {
	if a.legacyCodeFilename == "" {
		log.Println("Legacy key file not provided")
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

		u := User{Name: code, UserLevel: LEVEL_LEGACY, Codes: matches[1:]}
		a.validUsers[code] = &u
	}

}

//
//Read the user CSV file
//
//It is name, level, code[,code...]
func (a *Authenticator) readUserFile() {
	if a.userFilename == "" {
		log.Println("RFID-user file not provided")
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
		//comment
		if strings.TrimSpace(line[0])[0] == '#' {
			continue
		}
		u := User{Name: line[0], UserLevel: Level(line[1]), Codes: line[2:]}
		log.Println("Got a new user", u)

		for _, code := range u.Codes {
			log.Println(code)
			a.validUsers[code] = &u
		}
	}
}

// Check if access for a given code is granted to a given Target
func (a *Authenticator) AuthUser(code string, target Target) bool {
	u, ok := a.validUsers[code]
	if !ok {
		log.Println("code bad", code)
		return false
	}

	return a.LevelHasAccess(u.UserLevel, target)
}

// Certain levels only have access during the daytime
// This implements that logic, which is 10am - 10pm
func (a *Authenticator) isDaytime() bool {
	now := time.Now()
	now = now.In(local)
	hour, _, _ := now.Clock()
	return hour >= 10 && hour < 22

}

func (a *Authenticator) LevelHasAccess(level Level, target Target) bool {
	now := time.Now()

	now = now.In(local)
	// Members have access
	if level == LEVEL_MEMBER {
		return true
	} else if level == LEVEL_USER {
		return a.isDaytime()
	} else if level == LEVEL_LEGACY {
		return target == TARGET_DOWNSTAIRS && a.isDaytime()
	}

	return false
}
