package main

import (
	"encoding/csv"
	"io/ioutil"
	"syscall"
	"testing"
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
		UserLevel: "member",
		Codes:     []string{"root123"}}
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
		UserLevel: LevelUser,
		Codes:     []string{"doe123"}}
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
	u.Codes[0] = "other123"
	ExpectTrue(t, auth.AddNewUser("root123", u),
		"Adding another user with unique code.")

	// Ok, now let's see if an new authenticator can make sense of the
	// file we appended to.
	auth = NewFileBasedAuthenticator(authFile.Name(), "")
	ExpectTrue(t, auth.FindUser("root123") != nil, "Finding root123")
	ExpectTrue(t, auth.FindUser("doe123") != nil, "Finding doe123")
	ExpectTrue(t, auth.FindUser("other123") != nil, "Finding other123")
}
