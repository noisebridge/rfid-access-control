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

	// Given a code (RFID or PIN), does it exist and is the user allowed
	// to access "target" ?
	AuthUser(code string, target Target) (AuthResult, string)

	// Given a valid authentication code of some member (PIN or RFID), add
	/// the new user object. Updates the file.
	AddNewUser(authentication_code string, user User) (bool, string)

	// Given a valid authentication code of some member, find user by code
	// to update: the updater_fun callback is called with the current user
	// information. Within the function, the user can be modified.
	// If updater_fun returns true, database is updated.
	UpdateUser(authentication_code string, user_code string, updater_fun ModifyFun) (bool, string)

	// Given a valid authentication code of some member, delete user
	// associated with user_code.
	DeleteUser(authentication_code string, user_code string) (bool, string)
}

type FileBasedAuthenticator struct {
	userFilename  string
	fileTimestamp time.Time  // modification timestamp.
	fileLock      sync.Mutex // File writing

	// List of users and various indexes needed to look-up. Never use
	// directly, use the ...UserSyncronized() methods.
	// For modifications, we employ an optimistic concurrency control:
	// on change operations we determine if we are still in the same
	// revision when we looked up the item to change.
	userLock   sync.Mutex       // Mutex to protect following data structures
	userList   []*User          // Sequence of users
	user2index map[*User]int    // user-pointer to index in userList
	code2user  map[string]*User // access-code to user
	revision   int              // counter for optimistic locking.

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

	if !a.readDatabase() {
		return nil
	}
	return a
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

func (a *FileBasedAuthenticator) AddNewUser(authentication_code string, user User) (bool, string) {
	if auth_ok, auth_msg := a.verifyModifyOperationAllowed(authentication_code); !auth_ok {
		return false, auth_msg
	}

	// We remember the sponsor who added the user.
	user.Sponsors = []string{hashAuthCode(authentication_code)}
	// If no valid from date is given, then this is creation time.
	if user.ValidFrom.IsZero() {
		user.ValidFrom = a.clock.Now()
	}
	// Are the codes used unique ?
	if !a.addUserSynchronized(&user) {
		return false, "Duplicate codes while adding user"
	}

	return a.appendDatabaseSingleEntry(&user)
}

func (a *FileBasedAuthenticator) UpdateUser(authentication_code string,
	user_code string, updater_fun ModifyFun) (bool, string) {
	if auth_ok, auth_msg := a.verifyModifyOperationAllowed(authentication_code); !auth_ok {
		return false, auth_msg
	}

	var previous_revision int
	orig_user := a.findUserSynchronized(user_code, &previous_revision)
	modification_copy := *orig_user
	// Call back the caller asking for modification of this user record. We
	// hand out a copy to mess with. If updater_fun() decides to not modify
	// or discard the modification, it can return false and we abort.
	if !updater_fun(&modification_copy) {
		return false, "Upate abort."
	}

	// Alright, some modification has been done. Update, but make sure to
	// only do that if nothing has changed in the meantime.
	if !a.replaceUserSynchronized(previous_revision, orig_user, &modification_copy) {
		return false, "Changed while editing."
	}

	return a.writeDatabase()
}

func (a *FileBasedAuthenticator) DeleteUser(
	authentication_code string, user_code string) (bool, string) {
	if auth_ok, auth_msg := a.verifyModifyOperationAllowed(authentication_code); !auth_ok {
		return false, auth_msg
	}

	var revision int
	user := a.findUserSynchronized(user_code, &revision)
	if !a.deleteUserSynchronized(revision, user) {
		return false, "Delete failed"
	}

	return a.writeDatabase()
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

// Find user; this returns the raw pointer to the User and you really only
// should modify while holdling a lock.
// If you want to use the returned object to call a modification operation:
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

// Add user.
// Makes sure the data structure is synchronized.
func (a *FileBasedAuthenticator) addUserSynchronized(user *User) bool {
	a.userLock.Lock()
	defer a.userLock.Unlock()
	a.revision++
	return a.addUserAtPosRequiresLock(user, -1)
}

func (a *FileBasedAuthenticator) deleteUserSynchronized(expected_revision int, user *User) bool {
	a.userLock.Lock()
	defer a.userLock.Unlock()
	if a.revision != expected_revision {
		return false
	}
	a.revision++
	user_index := a.deleteUserRequiresLock(user)
	return user_index >= 0
}

// Replace user if the revision of the system is still the same as expected.
func (a *FileBasedAuthenticator) replaceUserSynchronized(expected_revision int, old_user *User, new_user *User) bool {
	a.userLock.Lock()
	defer a.userLock.Unlock()
	if a.revision != expected_revision {
		return false
	}
	a.revision++
	user_index := a.deleteUserRequiresLock(old_user)
	return a.addUserAtPosRequiresLock(new_user, user_index)
}

// Add a user at particular position. -1 for append.
// Requires the userLock to be held.
func (a *FileBasedAuthenticator) addUserAtPosRequiresLock(user *User, at_index int) bool {
	// ASSERT: a.userLock already locked.
	// First verify that there is no code in there that is already used by
	// someone else.
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
			// The caller messed up.
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

// Delete user and return index where it was.
func (a *FileBasedAuthenticator) deleteUserRequiresLock(user *User) int {
	// ASSERT: a.userLock already locked.
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

//
// Read the user CSV file
//
// It is name, level, code[,code...]
func (a *FileBasedAuthenticator) readDatabase() bool {
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

// For now, we sometimes need to modify the file manually, e.g. to add contact
// info. This allows to automatically reload it.
func (a *FileBasedAuthenticator) reloadIfChanged() {
	a.fileLock.Lock()
	defer a.fileLock.Unlock()
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

// Full dump of database.
func (a *FileBasedAuthenticator) writeDatabase() (bool, string) {
	// First, dump out the database to a temporary file and
	// make sure it succeeds.
	tmpFilename := a.userFilename + ".tmp"
	if ok, msg := a.writeTempCSV(tmpFilename); !ok {
		return false, msg
	}

	// Alright, good. Atomic rename.
	a.fileLock.Lock()
	defer a.fileLock.Unlock()
	os.Rename(tmpFilename, a.userFilename)

	fileinfo, _ := os.Stat(a.userFilename)
	a.fileTimestamp = fileinfo.ModTime()

	return true, ""
}

// Like write database, but just append a single user. In that case, a file
// append is sufficient.
func (a *FileBasedAuthenticator) appendDatabaseSingleEntry(user *User) (bool, string) {
	// Just append the user to the file which is sufficient for AddNewUser()
	a.fileLock.Lock()
	defer a.fileLock.Unlock()
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
func (a *FileBasedAuthenticator) writeTempCSV(filename string) (bool, string) {
	os.Remove(filename)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return false, err.Error()
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	for _, user := range a.userList {
		if user != nil {
			user.WriteCSV(writer)
		}
	}
	writer.Flush()
	/* writer.Error() does not exist in older go versions :(
	if writer.Error() != nil {
		log.Println(writer.Error())
		return false, writer.Error()
	}
	*/
	return true, ""
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
// their lengths while browsing the file. A weak MD5 is more than enough for
// this use-case.
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

func (a *FileBasedAuthenticator) levelHasAccess(level Level, target Target) (AuthResult, string) {
	// TODO: we need a concept of an 'open' space, i.e. a responsible user
	// opens the space to be accessible by the public, so that other users
	// can come in even outside 'their' times. Right now only dummy - never
	// open.
	space_open_to_public := false

	hour_from, hour_to := AccessHours(level)
	current_hour := a.clock.Now().Hour()
	isday := space_open_to_public ||
		(current_hour >= hour_from && current_hour < hour_to)
	switch level {
	case LevelMember:
		return AuthOk, "" // Members always have access.

	case LevelFulltimeUser:
		if !isday {
			return AuthOkButOutsideTime,
				fmt.Sprintf("Fulltime user outside %d:00..%d:00",
					hour_from, hour_to)
		}
		return AuthOk, ""

	case LevelUser:
		if !isday {
			return AuthOkButOutsideTime,
				fmt.Sprintf("Regular user outside %d:00..%d:00",
					hour_from, hour_to)
		}
		return AuthOk, ""

	case LevelHiatus:
		return AuthFail, "On Hiatus"
	}
	return AuthFail, ""
}
