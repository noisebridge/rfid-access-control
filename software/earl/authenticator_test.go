package main

import (
	"encoding/csv"
	"io/ioutil"
	"syscall"
	"testing"
	"time"
)

func ExpectTrue(t *testing.T, condition bool, message string) {
	if !condition {
		t.Errorf("Expected to fail, but didn't: %s", message)
	}
}

func ExpectFalse(t *testing.T, condition bool, message string) {
	if condition {
		t.Errorf("Expected to succeed, but didn't: %s", message)
	}
}

func TestAddUser(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "test-add-user")

	// Seed with one member
	authFile.WriteString("# Comment\n")
	authFile.WriteString("# This is a comment,with,multi,comma,foo,bar,x\n")
	rootUser := User{
		Name:      "root",
		UserLevel: "member"}
	rootUser.SetAuthCode("root123")
	writer := csv.NewWriter(authFile)
	rootUser.WriteCSV(writer)
	writer.Flush()
	authFile.Close()
	defer syscall.Unlink(authFile.Name())

	auth := NewFileBasedAuthenticator(authFile.Name(), "")

	found := auth.FindUser("doe123")
	ExpectTrue(t, found == nil, "Didn't expect non-existent code to work")

	u := User{
		Name:      "Jon Doe",
		UserLevel: LevelUser}
	ExpectFalse(t, u.SetAuthCode("short"), "Adding too short code")
	ExpectTrue(t, u.SetAuthCode("doe123"), "Adding long enough auth code")
	// Can't add with bogus member
	ExpectFalse(t, auth.AddNewUser("non-existent", u),
		"Adding new user with non-existent code.")

	// Proper member adding user.
	ExpectTrue(t, auth.AddNewUser("root123", u),
		"Add user with valid member account")

	// Now, freshly added, we should be able to find the user.
	found = auth.FindUser("doe123")
	if found == nil || found.Name != "Jon Doe" {
		t.Errorf("Didn't find user for code")
	}

	// Let's attempt to set a user with the same code
	ExpectFalse(t, auth.AddNewUser("root123", u),
		"Adding user with code already in use.")

	u.Name = "Another,user;[]funny\"characters '" // Stress-test CSV :)
	u.SetAuthCode("other123")
	ExpectTrue(t, auth.AddNewUser("root123", u),
		"Adding another user with unique code.")

	u.Name = "ExpiredUser"
	u.SetAuthCode("expired123")
	u.ValidTo = time.Now().Add(-1 * time.Hour)
	ExpectTrue(t, auth.AddNewUser("root123", u), "Adding user")

	// TODO: can't test AuthUser() yet as we need a simulated clock
	// to do daytime/nightime/expiredness checking.

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
