package main

// TODO
// add reloadIfChanged()
// return all auth errors not only as boolean but with string description, so
//  that it is easy to
import (
	"bufio"
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"io"
	"log"
	"os"
	"regexp"
	"sync"
)

type Authenticator interface {
	// Given a code, is the user allowed to access "target" ?
	AuthUser(code string, target Target) bool

	// Given the authenticator token (checked for memberness),
	// add the given user.
	// Updates the file
	AddNewUser(authentication_code string, user User) bool

	// Find a user for the given string. Returns a copy or 'nil' if the
	// user doesn't exist.
	FindUser(plain_code string) *User
}

type FileBasedAuthenticator struct {
	userFilename string
	// TODO: reload-if-changed by checking timestamp
	legacyCodeFilename string // We load this once, but don't expect changes

	// Map of codes to users. Quick way to look-up auth. Never use direclty,
	// use findUserSynchronized() and addUserSynchronized() for locking.
	validUsers     map[string]*User
	validUsersLock sync.Mutex

	clock Clock // Our source of time. Useful for simulated clock in tests
}

func NewFileBasedAuthenticator(userFilename string, legacyCodeFilename string) *FileBasedAuthenticator {
	a := &FileBasedAuthenticator{
		userFilename:       userFilename,
		legacyCodeFilename: legacyCodeFilename,
		validUsers:         make(map[string]*User),
		clock:              RealClock{},
	}

	a.validUsers = make(map[string]*User)
	a.readLegacyFile()
	a.readUserFile()
	return a
}

// We hash the authentication codes, as we don't need/want knowledge
// of actual IDs just to be able to verify.
//
// Note, this hash can _not_ protect against brute-force attacks; if you
// have the file, some CPU cycles and can emulate tokens, you are in
// (pin-codes are relatively short, and some older Mifare cards only have
// 32Bit IDs, so no protection against cheaply generated rainbow tables).
// But then again, you are more than welcome in a Hackerspace in that case :)
//
// So we merely protect against accidentally revealing a PIN or card-ID and
// their lengths while browsing the file.
func hashAuthCode(plain string) string {
	hashgen := md5.New()
	io.WriteString(hashgen, "MakeThisALittleBitLongerToChewOnEarlFoo"+plain)
	return hex.EncodeToString(hashgen.Sum(nil))
}

// Verify that code is long enough (and probably other syntactical things, such
// as not all the same digits and such)
func hasMinimalCodeRequirements(code string) bool {
	// 32Bit Mifare are 8 characters hex, this is to impose a minimum
	// 'strength' of a pin.
	return len(code) >= 6
}

// Find user. Synchronizes map.
func (a *FileBasedAuthenticator) findUserSynchronized(plain_code string) *User {
	a.validUsersLock.Lock()
	defer a.validUsersLock.Unlock()
	user, _ := a.validUsers[hashAuthCode(plain_code)]
	return user
}

// Add user to the internal data structure.
// Makes sure the data structure is synchronized.
func (a *FileBasedAuthenticator) addUserSynchronized(user *User) bool {
	a.validUsersLock.Lock()
	defer a.validUsersLock.Unlock()
	// First verify that there is no code in there that is already set..
	for _, code := range user.Codes {
		if a.validUsers[code] != nil {
			log.Printf("Ignoring multiple used code '%s'", code)
			return false // Existing user with that code
		}
	}
	// Then ok to add.
	for _, code := range user.Codes {
		a.validUsers[code] = user
	}
	return true
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
		if !hasMinimalCodeRequirements(code) {
			log.Printf("%s: Minimal criteria not met: '%s'", a.legacyCodeFilename, code)
			continue
		}

		u := User{
			Name:      code,
			UserLevel: LevelLegacy}
		u.SetAuthCode(code)
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
		a.addUserSynchronized(user)
	}
}

func (a *FileBasedAuthenticator) FindUser(plain_code string) *User {
	user := a.findUserSynchronized(plain_code)
	if user == nil {
		return nil
	}
	retval := *user // Copy, so that caller does not mess with state.
	// TODO: stash away the original pointer in the copy, which we then
	// use for update operation later. Once we have UpdateUser()
	return &retval
}

// TODO: return readable error instead of false.
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
	if !authMember.InValidityPeriod(a.clock.Now()) {
		log.Println("Member not in valid time-frame")
		return false
	}

	// TODO: Verify that there is some identifying information for the
	// user, otherwise only allow limited time range.

	// Right now, one sponsor, in the future we might require
	// a list depending on short/long-term expiry.
	user.Sponsors = []string{hashAuthCode(authentication_code)}
	// If no valid from date is given, then this is creation time.
	if user.ValidFrom.IsZero() {
		user.ValidFrom = a.clock.Now()
	}
	// Are the codes used unique ?
	if !a.addUserSynchronized(&user) {
		log.Printf("Duplicate codes while adding '%s'", user.Name)
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
	user.WriteCSV(writer)
	writer.Flush()
	return true
}

// Check if access for a given code is granted to a given Target
func (a *FileBasedAuthenticator) AuthUser(code string, target Target) bool {
	if !hasMinimalCodeRequirements(code) {
		log.Println("Auth failed: too short code.")
		return false
	}
	user := a.findUserSynchronized(code)
	if user == nil {
		log.Println("Auth requested; couldn't find user for code")
		return false
	}
	if !user.InValidityPeriod(a.clock.Now()) {
		log.Println("Code not valid yet/epxired")
		return false
	}
	return a.levelHasAccess(user.UserLevel, target)
}

// Certain levels only have access during the daytime
// This implements that logic, which is 11:00 - 21:59
func (a *FileBasedAuthenticator) isDaytime() bool {
	hour := a.clock.Now().Hour()
	return hour >= 11 && hour < 22
}

func (a *FileBasedAuthenticator) levelHasAccess(level Level, target Target) bool {
	switch level {
	case LevelMember:
		return true // Members always have access.
	case LevelUser:
		ok := a.isDaytime()
		if !ok {
			log.Println("Regular user outside daytime")
		}
		return ok
	case LevelLegacy:
		return a.isDaytime() && target == TargetDownstairs
	}

	return false
}
