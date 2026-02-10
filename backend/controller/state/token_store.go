package state

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type TokenRecord struct {
	Hash        string
	ExpiresAt   time.Time
	Used        bool
	ConnectorID string
}

type TokenStore struct {
	mu     sync.Mutex
	tokens map[string]*TokenRecord
	ttl    time.Duration
}

func NewTokenStore(ttl time.Duration) *TokenStore {
	return &TokenStore{
		tokens: make(map[string]*TokenRecord),
		ttl:    ttl,
	}
}

func (s *TokenStore) CreateToken() (string, time.Time, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(raw)
	hash := hashToken(token)
	expires := time.Now().Add(s.ttl)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[hash] = &TokenRecord{
		Hash:      hash,
		ExpiresAt: expires,
		Used:      false,
	}
	return token, expires, nil
}

func (s *TokenStore) ConsumeToken(token, connectorID string) error {
	if token == "" {
		return errors.New("missing token")
	}
	if connectorID == "" {
		return errors.New("missing connector id")
	}
	hash := hashToken(token)

	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tokens[hash]
	if !ok {
		return errors.New("invalid token")
	}
	if time.Now().After(rec.ExpiresAt) {
		return errors.New("token expired")
	}
	if rec.Used {
		return errors.New("token already used")
	}
	rec.Used = true
	rec.ConnectorID = connectorID
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
