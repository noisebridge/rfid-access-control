package main

// TODO
// add reloadIfChanged()

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
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
	// creation time ?
	// expire time ? (e.g. for one-day visitors with RFID or PIN-codes)
	// Who was/where the authenticator(s) for this user ?
}

// Create a new user read from a CSV reader
func NewUserFromCSV(reader *csv.Reader) (user *User, result_err error) {
	line, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		log.Println("Skipping short line", line)
		return nil, nil
	}
	// comment
	if strings.TrimSpace(line[0])[0] == '#' {
		return nil, nil
	}
	return &User{
			Name:      line[0],
			UserLevel: Level(line[1]),
			Codes:     line[2:]},
		nil
}

func (user *User) writeCSV(writer *csv.Writer) {
	var fields []string = make([]string, 2+len(user.Codes))
	fields[0] = user.Name
	fields[1] = string(user.UserLevel)
	for index, code := range user.Codes {
		fields[index+2] = code
	}
	writer.Write(fields)
}

type Authenticator interface {
	// Given a code, is the user allowed to access "target" ?
	AuthUser(code string, target Target) bool

	// Given the authenticator token (checked for memberness),
	// add the given user.
	// Updates the file
	AddNewUser(authentication_code string, user User) bool

	// Find a user for the given string. Returns a copy or 'nil' if the
	// user doesn't exist.
	FindUser(code string) *User
}

type FileBasedAuthenticator struct {
	userFilename string
	// TODO: reload-if-changed by checking timestamp
	legacyCodeFilename string // We load this once, but don't expect changes

	// Map of codes to users. Quick way to look-up auth. Never use direclty,
	// use findUserSynchronized() and addUserSynchronized() for locking.
	validUsers     map[string]*User
	validUsersLock sync.Mutex
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

// Find user. Synchronizes map.
func (a *FileBasedAuthenticator) findUserSynchronized(code string) *User {
	a.validUsersLock.Lock()
	defer a.validUsersLock.Unlock()
	user, _ := a.validUsers[code]
	return user
}

// Add user to the internal data structure.
// Makes sure the data structure is synchronized.
func (a *FileBasedAuthenticator) addUserSynchronized(user *User) bool {
	a.validUsersLock.Lock()
	defer a.validUsersLock.Unlock()
	all_codes_unique := true
	for _, code := range user.Codes {
		existing_user_with_code := a.validUsers[code]
		if existing_user_with_code == nil {
			log.Printf("Internally store '%s'", code)
			a.validUsers[code] = user
		} else {
			all_codes_unique = false
			log.Printf("Ignoring multiple used code '%s'", code)
		}
	}
	return all_codes_unique
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
		a.addUserSynchronized(&u)
	}
}

//
// Read the user CSV file
//
// It is name, level, code[,code...]
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
		user, err := NewUserFromCSV(reader)
		if err != nil {
			break
		}
		if user == nil {
			continue // e.g. due to comment or short line
		}
		log.Println("Read user", user)
		a.addUserSynchronized(user)
	}
}

func (a *FileBasedAuthenticator) FindUser(code string) *User {
	user := a.findUserSynchronized(code)
	if user == nil {
		return nil
	}
	retval := *user // Copy, so that caller does not mess with state.
	// TODO: stash away the original pointer in the copy, which we then
	// use for update operation later. Once we have UpdateUser()
	return &retval
}

func (a *FileBasedAuthenticator) AddNewUser(authentication_code string, user User) bool {
	// Only members can add.
	authMember := a.findUserSynchronized(authentication_code)
	if authMember == nil {
		log.Println("Couldn't find member with authentication code")
		return false
	}
	if authMember.UserLevel != LevelMember {
		log.Println("Non-member AddNewUser attempt")
		return false
	}
	// Are the codes used unique ?
	if !a.addUserSynchronized(&user) {
		log.Println("Duplicate codes")
		return false
	}

	// Just append the user to the file which is sufficient for AddNewUser()
	// TODO: When we allow for updates, we need to dump out the whole file
	// and do atomic rename.
	f, err := os.OpenFile(a.userFilename, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return false
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	user.writeCSV(writer)
	writer.Flush()
	log.Println("AddNewUser(): success")
	if writer.Error() != nil {
		log.Println(writer.Error())
	}
	return writer.Error() == nil
}

// Check if access for a given code is granted to a given Target
func (a *FileBasedAuthenticator) AuthUser(code string, target Target) bool {
	user := a.findUserSynchronized(code)
	if user == nil {
		log.Println("code bad", code)
		return false
	}

	return a.levelHasAccess(user.UserLevel, target)
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
