package main

import (
	"encoding/csv"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"syscall"
	"testing"
	"time"
)

const (
	keepGeneratedFiles = false // useful for debugging.
)

func ExpectTrue(t *testing.T, condition bool, message string) {
	if !condition {
		t.Errorf("Expected to succeed, but didn't: %s", message)
	}
}

func ExpectFalse(t *testing.T, condition bool, message string) {
	if condition {
		t.Errorf("Expected to fail, but didn't: %s", message)
	}
}

func ExpectResult(t *testing.T, auth AuthResult, msg string,
	expected_auth AuthResult, expected_re string, fail_prefix string) {
	matcher := regexp.MustCompile(expected_re)
	matches := matcher.MatchString(msg)
	if !matches {
		t.Errorf("%s: Expected '%s' to match '%s'",
			fail_prefix, msg, expected_re)
	}
	if auth != expected_auth {
		t.Errorf("%s: Expected %d, got %d", fail_prefix, expected_auth, auth)
	}
}

// Looks like I can't pass output of multiple return function as parameter-tuple.
// So doing this manually here.
func ExpectAuthResult(t *testing.T, auth Authenticator,
	code string, target Target,
	expected_auth AuthResult, expected_re string) {
	auth_result, msg := auth.AuthUser(code, target)
	ExpectResult(t, auth_result, msg, expected_auth, expected_re,
		code+","+string(target))
}

// Strip message from bool/string tuple and just return the bool
func eatmsg(ok bool, msg string) bool {
	if msg != "" {
		log.Printf("TEST: ignore msg '%s'", msg)
	}
	return ok
}

// File based authenticator we're working with. Seeded with one root-user
func CreateSimpleFileAuth(authFile *os.File, clock Clock) Authenticator {
	// Seed with one member
	authFile.WriteString("# Comment\n")
	authFile.WriteString("# This is a comment,with,multi,comma,foo,bar,x\n")
	rootUser := User{
		Name:        "root",
		ContactInfo: "root@nb",
		UserLevel:   "member"}
	rootUser.SetAuthCode("root123")
	writer := csv.NewWriter(authFile)
	rootUser.WriteCSV(writer)
	writer.Flush()
	authFile.Close()
	auth := NewFileBasedAuthenticator(authFile.Name(), NewApplicationBus())
	auth.clock = clock
	return auth
}

func TestAddUser(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "test-add-user")
	auth := CreateSimpleFileAuth(authFile, RealClock{})
	if !keepGeneratedFiles {
		defer syscall.Unlink(authFile.Name())
	}

	found := auth.FindUser("doe123")
	ExpectTrue(t, found == nil, "Didn't expect non-existent code to work")

	u := User{
		Name:      "Jon Doe",
		UserLevel: LevelUser}
	//ExpectFalse(t, u.SetAuthCode("short"), "Adding too short code")
	ExpectFalse(t, u.SetAuthCode("sho"), "Adding too short code")
	ExpectTrue(t, u.SetAuthCode("doe123"), "Adding long enough auth code")
	// Can't add with bogus member
	ExpectFalse(t, eatmsg(auth.AddNewUser("non-existent member", u)),
		"Adding new user with non-existent code.")

	// Proper member adding user.
	ExpectTrue(t, eatmsg(auth.AddNewUser("root123", u)),
		"Add user with valid member account")

	// Now, freshly added, we should be able to find the user.
	found = auth.FindUser("doe123")
	if found == nil || found.Name != "Jon Doe" {
		t.Errorf("Didn't find user for code")
	}

	// Let's attempt to set a user with the same code
	ExpectFalse(t, eatmsg(auth.AddNewUser("root123", u)),
		"Adding user with code already in use.")

	u.Name = "Another,user;[]funny\"characters '" // Stress-test CSV :)
	u.SetAuthCode("other123")
	ExpectTrue(t, eatmsg(auth.AddNewUser("root123", u)),
		"Adding another user with unique code.")

	u.Name = "ExpiredUser"
	u.SetAuthCode("expired123")
	u.ValidTo = time.Now().Add(-1 * time.Hour)
	ExpectTrue(t, eatmsg(auth.AddNewUser("root123", u)), "Adding user")

	// Attempt to add a user with a non-member auth code
	u.Name = "Shouldnotbeadded"
	u.SetAuthCode("shouldfail")
	ExpectFalse(t, eatmsg(auth.AddNewUser("doe123", u)),
		"John Doe may not add users")

	// Permission testing: see if regular users or philanthropist can
	// add users (which they shouldn't)
	// Let's add an Philanthropist as well.
	u.Name = "Joe Philanthropist"
	u.UserLevel = LevelPhilanthropist
	u.ValidTo = time.Now().Add(1 * time.Hour)
	u.ContactInfo = "phil@foo"
	u.SetAuthCode("phil123")
	auth.AddNewUser("root123", u)

	// Permission testing:
	u.Name = "Attempt Jon Doe adding"
	u.SetAuthCode("fromdoe")
	ExpectFalse(t, eatmsg(auth.AddNewUser("doe123", u)),
		"Attempt to add user by non-member")

	// A Philanthropist however, can add a new user.
	u.Name = "Philanthropist adding"
	u.SetAuthCode("fromphil")
	ExpectTrue(t, eatmsg(auth.AddNewUser("phil123", u)),
		"Philanthropist adding new user")

	// Ok, now let's see if an new authenticator can make sense of the
	// file we appended to.
	auth = NewFileBasedAuthenticator(authFile.Name(), NewApplicationBus())
	ExpectTrue(t, auth.FindUser("root123") != nil, "Finding root123")
	ExpectTrue(t, auth.FindUser("doe123") != nil, "Finding doe123")
	ExpectTrue(t, auth.FindUser("other123") != nil, "Finding other123")
	ExpectTrue(t, auth.FindUser("expired123") != nil, "Finding expired123")
}

func TestUpdateUser(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "test-update-user")
	auth := CreateSimpleFileAuth(authFile, RealClock{})
	if !keepGeneratedFiles {
		defer syscall.Unlink(authFile.Name())
	}

	u := User{
		Name:      "Jon Doe",
		UserLevel: LevelUser}
	u.SetAuthCode("doe123")
	auth.AddNewUser("root123", u)

	u.Name = "Unchanged User"
	u.SetAuthCode("unchanged123")
	auth.AddNewUser("root123", u)

	u.Name = "Jon Philanthropist"
	u.UserLevel = LevelPhilanthropist
	u.SetAuthCode("phil123")
	auth.AddNewUser("root123", u)

	ExpectTrue(t, auth.FindUser("doe123") != nil, "Old doe123")
	ExpectTrue(t, auth.FindUser("unchanged123") != nil, "Unchanged User")
	ExpectFalse(t, auth.FindUser("newdoe123") != nil, "Not yet newdoe123")

	// Regular user can't update
	ExpectFalse(t, eatmsg(auth.UpdateUser("doe123", "doe123", func(user *User) bool { return true })),
		"Regular user attempted to update")

	// .. but Philanthropist is allowed.
	ExpectTrue(t, eatmsg(auth.UpdateUser("phil123", "doe123", func(user *User) bool { return true })),
		"Philanthropist should be able to update")

	// Now let the root user modify user identified by doe123
	auth.UpdateUser("root123", "doe123", func(user *User) bool {
		user.SetAuthCode("newdoe123")
		user.ContactInfo = "hello@world"
		return true
	})

	ExpectFalse(t, auth.FindUser("doe123") != nil, "New, doe123 not valid anymore")

	updatedUser := auth.FindUser("newdoe123")
	ExpectTrue(t, updatedUser != nil, "Finding newdoe123")
	ExpectTrue(t, updatedUser.ContactInfo == "hello@world", "Finding newdoe123")

	// This guy should still be there and found.
	ExpectTrue(t, auth.FindUser("unchanged123") != nil, "Unchanged User")

	// Now let's see if everything is properly persisted
	auth = NewFileBasedAuthenticator(authFile.Name(), NewApplicationBus())
	ExpectTrue(t, auth.FindUser("root123") != nil, "Reread: Finding root123")
	ExpectTrue(t, auth.FindUser("unchanged123") != nil, "Reread: Finding unchanged123")
	ExpectTrue(t, auth.FindUser("newdoe123") != nil, "Reread: Finding newdoe123")
	updatedUser = auth.FindUser("newdoe123")
	ExpectTrue(t, updatedUser.ContactInfo == "hello@world", "Reread: contact newdoe123")
}

func TestDeleteUser(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "test-delete-user")
	auth := CreateSimpleFileAuth(authFile, RealClock{})
	if !keepGeneratedFiles {
		defer syscall.Unlink(authFile.Name())
	}

	u := User{
		Name:      "Jon Doe",
		UserLevel: LevelUser}
	u.SetAuthCode("doe123")
	auth.AddNewUser("root123", u)

	u.Name = "Unchanged User"
	u.SetAuthCode("unchanged123")
	auth.AddNewUser("root123", u)

	ExpectTrue(t, auth.FindUser("doe123") != nil, "Old doe123")
	ExpectTrue(t, auth.FindUser("unchanged123") != nil, "Unchanged User")

	// Now let the root user delete user identified by doe123
	auth.DeleteUser("root123", "doe123")

	ExpectFalse(t, auth.FindUser("doe123") != nil, "New, doe123 not valid anymore")
	// This guy should still be there and found.
	ExpectTrue(t, auth.FindUser("unchanged123") != nil, "Unchanged User")

	auth = NewFileBasedAuthenticator(authFile.Name(), NewApplicationBus())
	ExpectTrue(t, auth.FindUser("root123") != nil, "Reread: Finding root123")
	ExpectTrue(t, auth.FindUser("unchanged123") != nil, "Reread: Finding unchanged")
	ExpectFalse(t, auth.FindUser("doe123") != nil, "Reread: Finding doe123")
}

func TestTimeLimits(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "timing-tests")
	mockClock := &MockClock{}
	auth := CreateSimpleFileAuth(authFile, mockClock)
	if !keepGeneratedFiles {
		defer syscall.Unlink(authFile.Name())
	}

	someMidnight, _ := time.Parse("2006-01-02", "2014-10-10") // midnight
	nightTime_3h := someMidnight.Add(3 * time.Hour)           // 03:00
	earlyMorning_7h := someMidnight.Add(7 * time.Hour)        // 09:00
	hackerDaytime_13h := someMidnight.Add(13 * time.Hour)     // 16:00
	closingTime_22h := someMidnight.Add(22 * time.Hour)       // 22:00
	lateStayUsers_23h := someMidnight.Add(23 * time.Hour)     // 23:00

	// After 30 days, non-contact users expire.
	// So fast forward 31 days, 16:00 in the afternoon.
	anonExpiry_30d := someMidnight.Add(30*24*time.Hour + 16*time.Hour)

	// We 'register' the users a day before
	mockClock.now = someMidnight.Add(-12 * time.Hour)
	// Adding various users.
	u := User{
		Name:        "Some Member",
		ContactInfo: "member@noisebridge.net",
		UserLevel:   LevelMember}
	u.SetAuthCode("member123")
	auth.AddNewUser("root123", u)

	u = User{
		Name:        "Some User",
		ContactInfo: "user@noisebridge.net",
		UserLevel:   LevelUser}
	u.SetAuthCode("user123")
	auth.AddNewUser("root123", u)

	u = User{
		Name:        "Some Fulltime User",
		ContactInfo: "ftuser@noisebridge.net",
		UserLevel:   LevelFulltimeUser}
	u.SetAuthCode("fulltimeuser123")
	auth.AddNewUser("root123", u)

	u = User{
		Name:        "A Philanthropist",
		ContactInfo: "Philanthropist@noisebridge.net",
		UserLevel:   LevelPhilanthropist}
	u.SetAuthCode("philanthropist123")
	auth.AddNewUser("root123", u)

	u = User{
		Name:        "User on Hiatus",
		ContactInfo: "gone@fishing.net",
		UserLevel:   LevelHiatus}
	u.SetAuthCode("hiatus123")
	auth.AddNewUser("root123", u)

	// Member without contact info
	u = User{UserLevel: LevelMember}
	u.SetAuthCode("member_nocontact")
	auth.AddNewUser("root123", u)

	// User without contact info
	u = User{UserLevel: LevelUser}
	u.SetAuthCode("user_nocontact")
	auth.AddNewUser("root123", u)

	mockClock.now = nightTime_3h
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "philanthropist123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "fulltimeuser123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs,
		AuthOkButOutsideTime, "outside")

	mockClock.now = earlyMorning_7h
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "philanthropist123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "fulltimeuser123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs,
		AuthOkButOutsideTime, "outside")

	mockClock.now = hackerDaytime_13h
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "philanthropist123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "fulltimeuser123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "hiatus123", TargetUpstairs,
		AuthFail, "hiatus")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs, AuthOk, "")

	mockClock.now = closingTime_22h // should behave similar to earlyMorning
	ExpectAuthResult(t, auth, "philanthropist123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "fulltimeuser123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs,
		AuthOkButOutsideTime, "outside")

	mockClock.now = lateStayUsers_23h // members, philanthropists, and fulltimeusers left
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "philanthropist123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "fulltimeuser123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs,
		AuthOkButOutsideTime, "outside")

	// Automatic expiry of entries that don't have contact info
	mockClock.now = anonExpiry_30d
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "philanthropist123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "fulltimeuser123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs, AuthOk, "")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs,
		AuthExpired, "Code not valid yet/expired")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs,
		AuthExpired, "Code not valid yet/expired")
}

func TestHolidayTimeLimits(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "holiday-timing-tests")
	mockClock := &MockClock{}
	auth := CreateSimpleFileAuth(authFile, mockClock)
	if !keepGeneratedFiles {
		defer syscall.Unlink(authFile.Name())
	}

	someMidnight, _ := time.Parse("2006-01-02", "2016-12-24") // midnight
	nightTime_3h := someMidnight.Add(3 * time.Hour)           // 03:00
	earlyMorning_7h := someMidnight.Add(7 * time.Hour)        // 09:00
	hackerDaytime_13h := someMidnight.Add(13 * time.Hour)     // 16:00
	closingTime_22h := someMidnight.Add(22 * time.Hour)       // 22:00
	lateStayUsers_23h := someMidnight.Add(23 * time.Hour)     // 23:00

	// We 'register' the users a day before
	mockClock.now = someMidnight.Add(-12 * time.Hour)
	// Adding various users.

	u := User{
		Name:        "Some User",
		ContactInfo: "user@noisebridge.net",
		UserLevel:   LevelUser}
	u.SetAuthCode("user123")
	auth.AddNewUser("root123", u)

	mockClock.now = nightTime_3h
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")

	mockClock.now = earlyMorning_7h
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")

	mockClock.now = hackerDaytime_13h
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "holiday")

	mockClock.now = closingTime_22h // should behave similar to earlyMorning
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")

	mockClock.now = lateStayUsers_23h
	ExpectAuthResult(t, auth, "user123", TargetUpstairs,
		AuthOkButOutsideTime, "outside")
}
