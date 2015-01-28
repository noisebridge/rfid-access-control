// The Authenticator provides the storage of user information that knows about users and provide
// the interface to ask if a particular user is authenticated.
//
// This file defines the Authenticator interface, which is the simple API to be used by
// all the handlers that need to authenticate or modify users.
//
// This file also contains a concrete implementation (FileBasedAuthenticator) that stores users
// in a CSV file.
//
package main

// TODO
// - We need the concept of an 'open space'. If the space is open (e.g.
//   two members state that they are there), then regular users should come
//   in independent of time.
// - be able to write
import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

type AuthResult int

const (
	AuthFail             = AuthResult(0) // Not authorized.
	AuthExpired          = AuthResult(1)
	AuthOkButOutsideTime = AuthResult(2) // User ok; time-of-day limit.
	AuthOk               = AuthResult(42)
)

// Modify a user pointer. Returns 'true' if the changes should be written back.
type ModifyFun func(user *User) bool

type Authenticator interface {
	// Find a user for the given string. Returns a copy or 'nil' if the
	// user doesn't exist.
	FindUser(plain_code string) *User

	// Given a code (RFID or PIN), does it exist and is the user allowed to access "target" ?
	AuthUser(code string, target Target) (AuthResult, string)

	// Given a valid authentication code of some member (PIN or RFID), add the new user
	// object. Updates the file.
	AddNewUser(authentication_code string, user User) (bool, string)

	// Given a valid authentication code of some member, find user by code to update:
	// the updater_fun is called with the current user information.
	// Within the function, the user can be modified.
	// If updater_fun returns true, database is updated.
	UpdateUser(authentication_code string, user_code string, updater_fun ModifyFun) (bool, string)

	// Given a valid authentication code of some member, delete user associated with
	// user_code.
	DeleteUser(authentication_code string, user_code string) (bool, string)
}

type FileBasedAuthenticator struct {
	userFilename  string
	fileTimestamp time.Time // Timestamp at read time.

	// Map of codes to users. Quick way to look-up auth. Never use directly,
	// use findUserSynchronized() and addUserSynchronized() for locking.
	userList   []*User          // Sequence of users
	user2index map[*User]int    // user-pointer to index in userList
	code2user  map[string]*User // access-code to user
	userLock   sync.Mutex

	revision int // counter for optimistic locking.

	clock Clock // Our source of time. Useful for simulated clock in tests
}

func NewFileBasedAuthenticator(userFilename string) *FileBasedAuthenticator {
	a := &FileBasedAuthenticator{
		userFilename: userFilename,
		userList:     make([]*User, 0, 10),
		user2index:   make(map[*User]int),
		code2user:    make(map[string]*User),
		revision:     0,
		clock:        RealClock{},
	}

	if !a.readUserFile() {
		return nil
	}
	return a
}

// We hash the authentication codes, as we don't need/want knowledge
// of actual IDs just to be able to verify.
//
// Note, this hash can _not_ protect against brute-force attacks; if you
// have access to the file, some CPU cycles and can emulate tokens, you are in
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

// Verify that code is long enough (and possibly other syntactical things, such
// as not all the same digits and such)
func hasMinimalCodeRequirements(code string) bool {
	// 32Bit Mifare are 8 characters hex, this is more to impose a minimum
	// 'strength' of a pin.
	return len(code) >= 6
}

// Find user. Synchronizes map.
// If revision is non-nil, fills in the current revision.
func (a *FileBasedAuthenticator) findUserSynchronized(plain_code string, rev *int) *User {
	a.reloadIfChanged()
	a.userLock.Lock()
	defer a.userLock.Unlock()
	user, _ := a.code2user[hashAuthCode(plain_code)]
	if rev != nil {
		*rev = a.revision
	}
	return user
}

// Delete user and return index where it was
func (a *FileBasedAuthenticator) deleteUserRequiresLock(user *User) int {
	pos, found := a.user2index[user]
	if !found {
		return -1
	}

	a.userList[pos] = nil
	delete(a.user2index, user)
	for _, code := range user.Codes {
		delete(a.code2user, code)
	}
	return pos
}

// Add a user at particular position. Requires the userLock to be locked.
func (a *FileBasedAuthenticator) addUserAtPosRequiresLock(user *User, at_index int) bool {
	// First verify that there is no code in there that is already set..
	for _, code := range user.Codes {
		if a.code2user[code] != nil {
			log.Printf("Ignoring multiple used code '%s'", code)
			return false // Existing user with that code
		}
	}
	// Then ok to add.
	if at_index < 0 {
		a.userList = append(a.userList, user)
		a.user2index[user] = len(a.userList) - 1
	} else {
		if a.userList[at_index] != nil {
			// Someone internally messed up using this.
			log.Fatalf("Doh' spot is actually not empty (%d)", at_index)
		}
		a.userList[at_index] = user
		a.user2index[user] = at_index
	}
	for _, code := range user.Codes {
		a.code2user[code] = user
	}
	return true
}

func (a *FileBasedAuthenticator) appendUserRequiresLock(user *User) bool {
	return a.addUserAtPosRequiresLock(user, -1)
}

// Add user to the internal data structure.
// Makes sure the data structure is synchronized.
func (a *FileBasedAuthenticator) addUserSynchronized(user *User) bool {
	a.userLock.Lock()
	defer a.userLock.Unlock()
	a.revision++
	return a.appendUserRequiresLock(user)
}

// Update user if the revision of the system is still the same as expected.
func (a *FileBasedAuthenticator) updateUserSynchronized(expected_revision int, old_user *User, new_user *User) bool {
	a.userLock.Lock()
	defer a.userLock.Unlock()
	if a.revision != expected_revision {
		return false
	}
	a.revision++
	user_index := a.deleteUserRequiresLock(old_user)
	return a.addUserAtPosRequiresLock(new_user, user_index)
}

func (a *FileBasedAuthenticator) deleteUserSynchronized(user *User) bool {
	a.userLock.Lock()
	defer a.userLock.Unlock()
	a.revision++
	user_index := a.deleteUserRequiresLock(user)
	return user_index >= 0
}

//
// Read the user CSV file
//
// It is name, level, code[,code...]
func (a *FileBasedAuthenticator) readUserFile() bool {
	if a.userFilename == "" {
		log.Println("RFID-user file not provided")
		return false
	}
	f, err := os.Open(a.userFilename)
	if err != nil {
		log.Println("Could not read RFID user-file", err)
		return false
	}

	fileinfo, _ := os.Stat(a.userFilename)
	a.fileTimestamp = fileinfo.ModTime()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 //variable length fields

	counts := make(map[Level]int)
	total := 0
	log.Printf("Reading %s", a.userFilename)
	for {
		user, done := NewUserFromCSV(reader)
		if done {
			break
		}
		if user == nil {
			continue // e.g. due to comment or short line
		}
		a.addUserSynchronized(user)
		counts[user.UserLevel]++
		total++
	}
	log.Printf("Read %d users from %s", total, a.userFilename)
	for level, count := range counts {
		log.Printf("%13s %4d", level, count)
	}
	return true
}

// For now, we sometimes need to modify the file directly, e.g. to add contact
// info. This allows to automatically reload it.
func (a *FileBasedAuthenticator) reloadIfChanged() {
	fileinfo, err := os.Stat(a.userFilename)
	if err != nil {
		return // well, ok then.
	}
	if a.fileTimestamp == fileinfo.ModTime() {
		return // nothing to do.
	}
	log.Printf("Refreshing changed %s (%s -> %s)\n",
		a.userFilename,
		a.fileTimestamp.Format("2006-01-02 15:04:05"),
		fileinfo.ModTime().Format("2006-01-02 15:04:05"))

	// For now, we are doing it simple: just create
	// a new authenticator and steal the result.
	// If we allow to modify users in-memory, we need to make
	// sure that we don't replace contents while that is happening.
	newAuth := NewFileBasedAuthenticator(a.userFilename)
	if newAuth == nil {
		return
	}
	a.userLock.Lock()
	defer a.userLock.Unlock()
	// Steal all the fields :)
	a.fileTimestamp = newAuth.fileTimestamp
	a.userList = newAuth.userList
	a.user2index = newAuth.user2index
	a.code2user = newAuth.code2user
}

func (a *FileBasedAuthenticator) FindUser(plain_code string) *User {
	user := a.findUserSynchronized(plain_code, nil)
	if user == nil {
		return nil
	}
	retval := *user // Copy, so that caller does not mess with state.
	return &retval
}

// Check if access for a given code is granted to a given Target
func (a *FileBasedAuthenticator) AuthUser(code string, target Target) (AuthResult, string) {
	if !hasMinimalCodeRequirements(code) {
		return AuthFail, "Auth failed: too short code."
	}
	user := a.findUserSynchronized(code, nil)
	if user == nil {
		return AuthFail, "No user for code"
	}
	// In case of Hiatus users, be a bit more specific with logging: this
	// might be someone stolen a token of some person on leave or attempt
	// of a blocked user to get access.
	if user.UserLevel == LevelHiatus {
		return AuthFail, fmt.Sprintf("User on hiatus '%s <%s>'", user.Name, user.ContactInfo)
	}
	if !user.InValidityPeriod(a.clock.Now()) {
		return AuthExpired, "Code not valid yet/expired"
	}
	return a.levelHasAccess(user.UserLevel, target)
}

func (a *FileBasedAuthenticator) verifyModifyOperationAllowed(auth_code string) (bool, string) {
	// Only members can modify.
	authMember := a.findUserSynchronized(auth_code, nil)
	if authMember == nil {
		return false, "Couldn't find member with authentication code."
	}
	if authMember.UserLevel != LevelMember {
		return false, "Non-member modify attempt"
	}
	if !authMember.InValidityPeriod(a.clock.Now()) {
		return false, "Auth-Member expired."
	}
	return true, ""
}

func (a *FileBasedAuthenticator) AddNewUser(authentication_code string, user User) (bool, string) {
	if auth_ok, auth_msg := a.verifyModifyOperationAllowed(authentication_code); !auth_ok {
		return false, auth_msg
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
		return false, "Duplicate codes while adding user"
	}

	// Just append the user to the file which is sufficient for AddNewUser()
	// TODO: When we allow for updates, we need to dump out the whole file
	// and do atomic rename.
	f, err := os.OpenFile(a.userFilename, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return false, err.Error()
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	user.WriteCSV(writer)
	writer.Flush()

	fileinfo, _ := os.Stat(a.userFilename)
	a.fileTimestamp = fileinfo.ModTime()

	return true, ""
}

// Write content of the 'user database' to temp CSV file.
func (a *FileBasedAuthenticator) writeTempCSV(filename string) bool {
	os.Remove(filename)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return false
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	for _, user := range a.userList {
		if user != nil {
			user.WriteCSV(writer)
		}
	}
	writer.Flush()
	/* writer.Error() does not exist in older go versions.
	if writer.Error() != nil {
		log.Println(writer.Error())
		return false
	}
	*/
	return true
}

func (a *FileBasedAuthenticator) writeDatabase() bool {
	tmpFilename := a.userFilename + ".tmp"
	if !a.writeTempCSV(tmpFilename) {
		return false
	}
	os.Rename(tmpFilename, a.userFilename)
	fileinfo, _ := os.Stat(a.userFilename)
	a.fileTimestamp = fileinfo.ModTime()

	return true
}

func (a *FileBasedAuthenticator) UpdateUser(authentication_code string,
	user_code string, updater_fun ModifyFun) (bool, string) {
	if auth_ok, auth_msg := a.verifyModifyOperationAllowed(authentication_code); !auth_ok {
		return false, auth_msg
	}

	var previous_revision int
	orig_user := a.findUserSynchronized(user_code, &previous_revision)
	modification_copy := *orig_user
	// Call back the caller asking for modification of this user record. We hand out
	// a copy to mess with. If updater_fun() decides to not modify or discard the
	// modification, it can return false.
	if !updater_fun(&modification_copy) {
		return false, "Upate abort"
	}

	// Alright, some modification has been done. Update but make sure to only do that if
	// nothing has changed in the meantime.
	if !a.updateUserSynchronized(previous_revision, orig_user, &modification_copy) {
		return false, "Changed while editing"
	}

	return a.writeDatabase(), ""
}

func (a *FileBasedAuthenticator) DeleteUser(
	authentication_code string, user_code string) (bool, string) {
	if auth_ok, auth_msg := a.verifyModifyOperationAllowed(authentication_code); !auth_ok {
		return false, auth_msg
	}

	user := a.findUserSynchronized(user_code, nil)
	if !a.deleteUserSynchronized(user) {
		return false, "Delete failed"
	}

	return a.writeDatabase(), ""
}

// Certain levels only have access during the daytime
// This implements that logic, which is 11:00 - 21:59
func (a *FileBasedAuthenticator) isUserDaytime() bool {
	hour := a.clock.Now().Hour()
	return hour >= 11 && hour < 22 // 11:00..21:59
}
func (a *FileBasedAuthenticator) isFulltimeUserDaytime() bool {
	hour := a.clock.Now().Hour()
	return hour >= 7 && hour <= 23 // 7:00..23:59
}

func (a *FileBasedAuthenticator) levelHasAccess(level Level, target Target) (AuthResult, string) {
	switch level {
	case LevelMember:
		return AuthOk, "" // Members always have access.

	case LevelFulltimeUser:
		isday := a.isFulltimeUserDaytime()
		if !isday {
			return AuthOkButOutsideTime,
				"Fulltime user outside daytime."
		}
		return AuthOk, ""

	case LevelUser:
		// TODO: we might want to make this dependent simply on
		// members having 'opened' the space.
		isday := a.isUserDaytime()
		if !isday {
			return AuthOkButOutsideTime,
				"Regular user outside daytime."
		}
		return AuthOk, ""

	case LevelHiatus:
		return AuthFail, "On Hiatus"
	}
	return AuthFail, ""
}
