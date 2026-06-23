package service

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"

	"crypto/pbkdf2"
)

const (
	saltBytes = 16
	hashBytes = 32
)

func HashPassword(password string, iterations int) ([]byte, []byte, error) {
	if iterations <= 0 {
		return nil, nil, fmt.Errorf("iterations must be positive")
	}
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, iterations, hashBytes)
	return salt, hash, err
}

func VerifyPassword(password string, salt, expected []byte, iterations int) (bool, error) {
	if len(salt) == 0 || len(expected) == 0 || iterations <= 0 {
		return false, nil
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false, err
	}
	if len(got) != len(expected) {
		return false, nil
	}
	return subtle.ConstantTimeCompare(got, expected) == 1 && !bytes.Equal(got, nil), nil
}
