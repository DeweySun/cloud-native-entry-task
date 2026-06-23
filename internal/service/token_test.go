package service

import (
	"testing"
	"time"
)

func TestTokenIssueVerify(t *testing.T) {
	manager, err := NewTokenManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	now := time.Unix(1000, 0)
	token, _, err := manager.Issue(42, now)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	userID, err := manager.Verify(token, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if userID != 42 {
		t.Fatalf("user id = %d, want 42", userID)
	}
	if _, err := manager.Verify(token, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expected expired token to fail")
	}
}
