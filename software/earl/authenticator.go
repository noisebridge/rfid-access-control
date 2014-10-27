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
)

var local *time.Location

type Level string

const (
	LevelLegacy = Level("legacy") // Legacy gate
	LevelUser   = Level("user")
	LevelMember = Level("member")
)

type User struct {
	Name      string
	UserLevel Level
	Codes     []string
}

type Authenticator interface {
	AuthUser(code string, target Target) bool
}

type FileBasedAuthenticator struct {
	userFilename       string
	legacyCodeFilename string
	lastChange         time.Time // last file timestamp we know; reload if file is newer
	validUsers         map[string]*User
}

func NewFileBasedAuthenticator(userFilename string, legacyCodeFilename string) *FileBasedAuthenticator {
	asdf, err := time.LoadLocation("America/Los_Angeles")
	local = asdf
	if err != nil {
		log.Fatal("Time zone death failure bad", err)
	}

	a := new(FileBasedAuthenticator)
	a.userFilename = userFilename
	a.legacyCodeFilename = legacyCodeFilename

	a.validUsers = make(map[string]*User)
	a.readLegacyFile()
	a.readUserFile()
	return a
}

func (a *FileBasedAuthenticator) readLegacyFile() {
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

		u := User{Name: code, UserLevel: LevelLegacy, Codes: matches[1:]}
		a.validUsers[code] = &u
	}
}

//
//Read the user CSV file
//
//It is name, level, code[,code...]
func (a *FileBasedAuthenticator) readUserFile() {
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
func (a *FileBasedAuthenticator) AuthUser(code string, target Target) bool {
	u, ok := a.validUsers[code]
	if !ok {
		log.Println("code bad", code)
		return false
	}

	return a.levelHasAccess(u.UserLevel, target)
}

// Certain levels only have access during the daytime
// This implements that logic, which is 10am - 10pm
func (a *FileBasedAuthenticator) isDaytime() bool {
	now := time.Now()
	now = now.In(local)
	hour, _, _ := now.Clock()
	return hour >= 10 && hour < 22
}

func (a *FileBasedAuthenticator) levelHasAccess(level Level, target Target) bool {
	switch level {
	case LevelMember:
		return true // Members always have access.
	case LevelUser:
		return a.isDaytime()
	case LevelLegacy:
		return a.isDaytime() && target == TargetDownstairs
	}

	return false
}
