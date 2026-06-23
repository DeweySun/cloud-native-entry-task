package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

type tokenClaims struct {
	UserID uint64 `json:"uid"`
	Exp    int64  `json:"exp"`
	Nonce  string `json:"nonce"`
}

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("token secret is required")
	}
	decoded, err := hex.DecodeString(secret)
	if err != nil {
		decoded = []byte(secret)
	}
	if len(decoded) < 32 {
		return nil, errors.New("token secret must be at least 32 bytes")
	}
	return &TokenManager{secret: decoded, ttl: ttl}, nil
}

func (m *TokenManager) Issue(userID uint64, now time.Time) (string, int64, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", 0, err
	}
	exp := now.Add(m.ttl).Unix()
	claims := tokenClaims{
		UserID: userID,
		Exp:    exp,
		Nonce:  base64.RawURLEncoding.EncodeToString(nonce),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", 0, err
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	sig := m.sign(payloadPart)
	return payloadPart + "." + sig, exp, nil
}

func (m *TokenManager) Verify(token string, now time.Time) (uint64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, ErrUnauthorized
	}
	expected := m.sign(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return 0, ErrUnauthorized
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, ErrUnauthorized
	}
	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, ErrUnauthorized
	}
	if claims.UserID == 0 || claims.Exp <= now.Unix() {
		return 0, ErrUnauthorized
	}
	return claims.UserID, nil
}

func (m *TokenManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	mac.Write([]byte(":v1:"))
	mac.Write([]byte(strconv.Itoa(len(payload))))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *TokenManager) TTL() time.Duration {
	return m.ttl
}

func tokenDebugString(userID uint64) string {
	return fmt.Sprintf("user:%d", userID)
}
