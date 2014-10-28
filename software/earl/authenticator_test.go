package main

import (
	"io/ioutil"
	"syscall"
	"testing"
)

func ExpectTrue(t *testing.T, condition bool, message string) {
	if !condition {
		t.Errorf(message)
	}
}

func ExpectFalse(t *testing.T, condition bool, message string) {
	if condition {
		t.Errorf(message)
	}
}

func TestAddUser(t *testing.T) {
	authFile, _ := ioutil.TempFile("", "test-add-user")

	// Seed with one member
	authFile.WriteString("root,member,root123\n")
	authFile.Close()
	defer syscall.Unlink(authFile.Name())

	auth := NewFileBasedAuthenticator(authFile.Name(), "")

	found := auth.FindUser("doe123")
	ExpectTrue(t, found == nil, "Didn't expect non-existent code to work")

	u := User{Name: "Jon Doe", UserLevel: LevelUser, Codes: []string{"doe123"}}
	// Can't add with bogus member
	ExpectFalse(t, auth.AddNewUser("non-existent", u),
		"Adding new user with non-existent code succeded")

	// Proper member adding user.
	ExpectTrue(t, auth.AddNewUser("root123", u),
		"Failed to add user with valid member account")

	// Now, freshly added, we should be able to find the user.
	found = auth.FindUser("doe123")
	if found == nil || found.Name != "Jon Doe" {
		t.Errorf("Didn't find user for pin")
	}

	// Let's attempt to set a user with the same code
	u.Name = "AnotherUser"
	ExpectFalse(t, auth.AddNewUser("root123", u),
		"Adding user with code already in use succeeded")

	u.Codes[0] = "other123"
	ExpectTrue(t, auth.AddNewUser("root123", u),
		"Adding user with disjoint code failed")

	// Ok, now let's see if an new authenticator can make sense of the
	// file.
	auth = NewFileBasedAuthenticator(authFile.Name(), "")
	ExpectTrue(t, auth.FindUser("root123") != nil, "Missing user root")
	ExpectTrue(t, auth.FindUser("doe123") != nil, "Missing doe")
	ExpectTrue(t, auth.FindUser("other123") != nil, "Missing Other")
}
