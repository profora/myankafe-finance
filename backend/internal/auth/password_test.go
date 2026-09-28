package auth

import "testing"

func TestPasswordHashVerify(t *testing.T) {
	password := "correct horse battery staple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	match, needs, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Fatal("expected password match")
	}
	if needs {
		t.Fatal("fresh hash should not need rehash")
	}
	match, _, err = VerifyPassword("wrong-password-value", hash)
	if err != nil {
		t.Fatal(err)
	}
	if match {
		t.Fatal("wrong password matched")
	}
}

func TestPasswordValidation(t *testing.T) {
	if ValidatePassword("too-short") == nil {
		t.Fatal("expected short password rejection")
	}
	if ValidatePassword("long-enough-password") == nil {
	} else {
		t.Fatal("expected valid password")
	}
}
