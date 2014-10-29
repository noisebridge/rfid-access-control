package main

// TODO
// add reloadIfChanged()

import (
	"bufio"
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
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
	// Name of user.
	// - Can be empty for time-limited anonymous codes
	// - Members should have a name they go by and can be recognized by
	//   others.
	// - Longer term tokens should also have a name to be able to do
	//   revocations on lost/stolen tokens or excluded visitors.
	Name        string    // Name to go by
	ContactInfo string    // Way to contact user (if set, should be unique)
	UserLevel   Level     // Level of access
	Sponsors    []string  // A list of sponsor codes when adding/updating
	ValidFrom   time.Time // E.g. for temporary classes pin
	ValidTo     time.Time // for anonymous tokens, day visitors or temp PIN
	Codes       []string  // List of PIN/RFID codes associated with user
	// Codes: one obfuscation step and only store hashes ?
	//   (to not accidentally pick up ID-info when browsing the file)
}

// User CSV
// Fields are stored in the sequence as they appear in the struct, with arrays
// being represented as semicolon separated lists.
// Create a new user read from a CSV reader
func NewUserFromCSV(reader *csv.Reader) (user *User, result_err error) {
	line, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if len(line) != 7 {
		log.Println("Skipping short line", line)
		// TODO: add legacy transformation.
		return nil, nil
	}
	// comment
	if strings.TrimSpace(line[0])[0] == '#' {
		return nil, nil
	}
	// TODO: not sure if this does proper locale matching
	ValidFrom, _ := time.Parse("2006-01-02 15:04", line[4])
	ValidTo, _ := time.Parse("2006-01-02 15:04", line[5])
	return &User{
			Name:        line[0],
			ContactInfo: line[1],
			UserLevel:   Level(line[2]),
			Sponsors:    strings.Split(line[3], ";"),
			ValidFrom:   ValidFrom, // field 4
			ValidTo:     ValidTo,   // field 5
			Codes:       strings.Split(line[6], ";")},
		nil
}

func (user *User) WriteCSV(writer *csv.Writer) {
	var fields []string = make([]string, 7)
	fields[0] = user.Name
	fields[1] = user.ContactInfo
	fields[2] = string(user.UserLevel)
	fields[3] = strings.Join(user.Sponsors, ";")
	if !user.ValidFrom.IsZero() {
		fields[4] = user.ValidFrom.Format("2006-01-02 15:04")
	}
	if !user.ValidTo.IsZero() {
		fields[5] = user.ValidTo.Format("2006-01-02 15:04")
	}
	fields[6] = strings.Join(user.Codes, ";")
	writer.Write(fields)
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

// Set the auth code to some value (should probably be add-auth-code)
// Returns true if code is long enough to meet criteria.
func (user *User) SetAuthCode(code string) bool {
	// 32Bit Mifare are 8 characters hex, this is to impose a minimum
	// 'strength' of a pin.
	if len(code) < 6 {
		return false
	}
	user.Codes = []string{hashAuthCode(code)}
	return true
}

func (user *User) InValidityPeriod() bool {
	return (user.ValidFrom.IsZero() || user.ValidFrom.Before(time.Now())) &&
		(user.ValidTo.IsZero() || user.ValidTo.After(time.Now()))
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
	if !authMember.InValidityPeriod() {
		return false
	}

	// TODO: Verify that there is some identifying information for the
	// user, otherwise only allow limited time range.

	// Right now, one sponsor, in the future we might require
	// a list depending on short/long-term expiry.
	user.Sponsors = []string{hashAuthCode(authentication_code)}
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
	user.WriteCSV(writer)
	writer.Flush()
	log.Println("AddNewUser()")
	return true
}

// Check if access for a given code is granted to a given Target
func (a *FileBasedAuthenticator) AuthUser(code string, target Target) bool {
	user := a.findUserSynchronized(code)
	if user == nil {
		log.Println("code bad", code)
		return false
	}
	if !user.InValidityPeriod() {
		log.Println("Code not valid yet/epxired")
		return false
	}
	return a.levelHasAccess(user.UserLevel, target)
}

// Certain levels only have access during the daytime
// This implements that logic, which is 11am - 10pm
func (a *FileBasedAuthenticator) isDaytime() bool {
	now := time.Now()
	now = now.In(local)
	hour, _, _ := now.Clock()
	return hour >= 11 && hour < 22
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
