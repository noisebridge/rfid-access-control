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

func ExpectResult(t *testing.T, ok bool, msg string,
	expected_ok bool, expected_re string, fail_prefix string) {
	matcher := regexp.MustCompile(expected_re)
	matches := matcher.MatchString(msg)
	if !matches {
		t.Errorf("%s: Expected '%s' to match '%s'",
			fail_prefix, msg, expected_re)
	}
	if ok != expected_ok {
		t.Errorf("%s: Expected %t, got %t", fail_prefix, expected_ok, ok)
	}
}

// Looks like I can't pass output of multiple return function as parameter-tuple.
// So doing this manually here.
func ExpectAuthResult(t *testing.T, auth Authenticator,
	code string, target Target,
	expected_ok bool, expected_re string) {
	ok, msg := auth.AuthUser(code, target)
	ExpectResult(t, ok, msg, expected_ok, expected_re,
		code+","+string(target))
}

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
	auth := NewFileBasedAuthenticator(authFile.Name(), "")
	auth.clock = clock
	return auth
}

func TestAddUser(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "test-add-user")
	auth := CreateSimpleFileAuth(authFile, RealClock{})
	defer syscall.Unlink(authFile.Name())

	found := auth.FindUser("doe123")
	ExpectTrue(t, found == nil, "Didn't expect non-existent code to work")

	u := User{
		Name:      "Jon Doe",
		UserLevel: LevelUser}
	ExpectFalse(t, u.SetAuthCode("short"), "Adding too short code")
	ExpectTrue(t, u.SetAuthCode("doe123"), "Adding long enough auth code")
	// Can't add with bogus member
	ExpectFalse(t, eatmsg(auth.AddNewUser("non-existent", u)),
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

	// Ok, now let's see if an new authenticator can make sense of the
	// file we appended to.
	auth = NewFileBasedAuthenticator(authFile.Name(), "")
	ExpectTrue(t, auth.FindUser("root123") != nil, "Finding root123")
	ExpectTrue(t, auth.FindUser("doe123") != nil, "Finding doe123")
	ExpectTrue(t, auth.FindUser("other123") != nil, "Finding other123")
	ExpectTrue(t, auth.FindUser("expired123") != nil, "Finding expired123")
}

func TestAuthLegacy(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "test-legacy")

	// Seed with one member
	authFile.WriteString("# Comment\n")
	authFile.WriteString("# This is a comment,with,multi,comma,foo,bar,x\n")
	authFile.WriteString("1234567  # some good pin\n")
	authFile.WriteString("1234     # too short pin\n")
	authFile.Close()
	defer syscall.Unlink(authFile.Name())

	auth := NewFileBasedAuthenticator("/dev/null", authFile.Name())

	found := auth.FindUser("1234")
	ExpectTrue(t, found == nil, "Too short, cannot be found")

	found = auth.FindUser("1234567")
	ExpectTrue(t, found != nil, "Too short, cannot be found")
}

func TestTimeLimits(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "timing-tests")
	mockClock := &MockClock{}
	auth := CreateSimpleFileAuth(authFile, mockClock)
	defer syscall.Unlink(authFile.Name())

	someMidnight, _ := time.Parse("2006-01-02", "2014-10-10") // midnight
	registerTime := someMidnight.Add(8 * time.Hour)           // 08:00
	earlyMorning := someMidnight.Add(9 * time.Hour)           // 09:00
	hackerDaytime := someMidnight.Add(16 * time.Hour)         // 16:00
	closingTime := someMidnight.Add(22 * time.Hour)           // 22:00
	// After 30 days, non-contact users expire.
	// So fast forward 31 days, 16:00 in the afternoon.
	anonExpiry := someMidnight.Add(31*24*time.Hour + 16*time.Hour)

	mockClock.now = registerTime
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

	// Member without contact info
	u = User{UserLevel: LevelMember}
	u.SetAuthCode("member_nocontact")
	auth.AddNewUser("root123", u)

	// User without contact info
	u = User{UserLevel: LevelUser}
	u.SetAuthCode("user_nocontact")
	auth.AddNewUser("root123", u)

	u = User{UserLevel: LevelLegacy}
	u.SetAuthCode("gate1234567")
	auth.AddNewUser("root123", u)

	mockClock.now = earlyMorning
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs, false,
		"outside daytime")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs, false,
		"outside daytime")
	ExpectAuthResult(t, auth, "gate1234567", TargetUpstairs, false,
		"Gate user outside daytime")
	ExpectAuthResult(t, auth, "gate1234567", TargetDownstairs, false,
		"Gate user outside daytime")

	mockClock.now = hackerDaytime
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "gate1234567", TargetUpstairs, false, "")
	ExpectAuthResult(t, auth, "gate1234567", TargetDownstairs, true, "")

	mockClock.now = closingTime // should behave similar to earlyMorning
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs, false,
		"outside daytime")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs, false,
		"outside daytime")
	ExpectAuthResult(t, auth, "gate1234567", TargetUpstairs, false,
		"Gate user outside daytime")
	ExpectAuthResult(t, auth, "gate1234567", TargetDownstairs, false,
		"Gate user outside daytime")

	mockClock.now = anonExpiry
	ExpectAuthResult(t, auth, "member123", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "user123", TargetUpstairs, true, "")
	ExpectAuthResult(t, auth, "member_nocontact", TargetUpstairs, false,
		"Code not valid yet/expired")
	ExpectAuthResult(t, auth, "user_nocontact", TargetUpstairs, false,
		"Code not valid yet/expired")
	ExpectAuthResult(t, auth, "gate1234567", TargetUpstairs, false, "")
	ExpectAuthResult(t, auth, "gate1234567", TargetDownstairs, false,
		"Code not valid yet/expired")
}
