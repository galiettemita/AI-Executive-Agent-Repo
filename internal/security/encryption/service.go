package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"sort"
	"sync"
)

const (
	nonceSize  = 12 // AES-GCM standard nonce size
	keySize    = 32 // AES-256 key size
)

// EncryptionService provides AES-256-GCM field-level encryption with key rotation support.
// Ciphertext format: nonce (12 bytes) || ciphertext || GCM tag.
type EncryptionService struct {
	mu        sync.Mutex
	keys      map[string][]byte // keyVersion -> raw key
	activeKey string            // version used for new encryptions
}

// NewEncryptionService creates a new encryption service.
func NewEncryptionService() *EncryptionService {
	return &EncryptionService{
		keys: map[string][]byte{},
	}
}

// AddKey registers an encryption key for the given version.
// The key must be exactly 32 bytes (AES-256).
func (s *EncryptionService) AddKey(keyVersion string, key []byte) error {
	if keyVersion == "" {
		return fmt.Errorf("key version must not be empty")
	}
	if len(key) != keySize {
		return fmt.Errorf("key must be %d bytes, got %d", keySize, len(key))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cp := make([]byte, keySize)
	copy(cp, key)
	s.keys[keyVersion] = cp
	return nil
}

// SetActiveKey sets which key version to use for new encryptions.
func (s *EncryptionService) SetActiveKey(keyVersion string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[keyVersion]; !ok {
		return fmt.Errorf("key version not found: %s", keyVersion)
	}
	s.activeKey = keyVersion
	return nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the active key.
// Returns ciphertext (nonce || encrypted || tag) and the key version used.
func (s *EncryptionService) Encrypt(plaintext []byte) ([]byte, string, error) {
	s.mu.Lock()
	activeVersion := s.activeKey
	key, ok := s.keys[activeVersion]
	s.mu.Unlock()

	if !ok || activeVersion == "" {
		return nil, "", fmt.Errorf("no active encryption key configured")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce prefix.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, activeVersion, nil
}

// Decrypt decrypts ciphertext using the key identified by keyVersion.
// Expects ciphertext format: nonce (12 bytes) || encrypted || tag.
func (s *EncryptionService) Decrypt(ciphertext []byte, keyVersion string) ([]byte, error) {
	s.mu.Lock()
	key, ok := s.keys[keyVersion]
	s.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("key version not found: %s", keyVersion)
	}

	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// RotateKey adds a new key and sets it as the active key for new encryptions.
// Old keys remain available for decryption (dual-key read window).
func (s *EncryptionService) RotateKey(newVersion string, newKey []byte) error {
	if err := s.AddKey(newVersion, newKey); err != nil {
		return fmt.Errorf("rotate key: %w", err)
	}
	if err := s.SetActiveKey(newVersion); err != nil {
		return fmt.Errorf("rotate key: %w", err)
	}
	return nil
}

// ListKeyVersions returns all registered key version identifiers, sorted.
func (s *EncryptionService) ListKeyVersions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions := make([]string, 0, len(s.keys))
	for v := range s.keys {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	return versions
}
