package state

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	path   string
}

func NewTokenStore(ttl time.Duration, path string) *TokenStore {
	store := &TokenStore{
		tokens: make(map[string]*TokenRecord),
		ttl:    ttl,
		path:   path,
	}
	_ = store.load()
	return store
}

func (s *TokenStore) CreateToken() (string, time.Time, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(raw)
	hash := hashToken(token)
	expires := time.Time{}
	if s.ttl > 0 {
		expires = time.Now().Add(s.ttl)
	} else {
		expires = time.Now().Add(10 * 365 * 24 * time.Hour)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[hash] = &TokenRecord{
		Hash:      hash,
		ExpiresAt: expires,
		Used:      false,
	}
	if err := s.saveLocked(); err != nil {
		return "", time.Time{}, err
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
	if !rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt) {
		return errors.New("token expired")
	}
	rec.ConnectorID = connectorID
	return s.saveLocked()
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *TokenStore) load() error {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var records map[string]*TokenRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = records
	return nil
}

func (s *TokenStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
