package service

import "testing"

func TestPasswordHashVerify(t *testing.T) {
	salt, hash, err := HashPassword("secret", 1000)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	ok, err := VerifyPassword("secret", salt, hash, 1000)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}
	ok, err = VerifyPassword("wrong", salt, hash, 1000)
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}
