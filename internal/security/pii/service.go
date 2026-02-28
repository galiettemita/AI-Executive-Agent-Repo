package pii

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

type CipherRecord struct {
	Field      string `json:"field"`
	Ciphertext string `json:"ciphertext"`
	Nonce      string `json:"nonce"`
	KeyVersion string `json:"key_version"`
}

type FieldPolicy struct {
	Field    string `json:"field"`
	Required bool   `json:"required"`
}

type legacyKey struct {
	key       []byte
	expiresAt time.Time
}

type Service struct {
	mu             sync.RWMutex
	currentKey     []byte
	keyVersion     string
	legacyKeys     map[string]legacyKey
	fieldPolicies  map[string]FieldPolicy
	rotationWindow time.Duration
	now            func() time.Time
}

func NewService() *Service {
	return &Service{
		currentKey:     []byte("0123456789abcdef0123456789abcdef"),
		keyVersion:     "v1",
		legacyKeys:     map[string]legacyKey{},
		fieldPolicies:  map[string]FieldPolicy{},
		rotationWindow: 10 * time.Minute,
		now:            func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetFieldPolicy(policy FieldPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(policy.Field) == "" {
		return
	}
	s.fieldPolicies[policy.Field] = policy
}

func (s *Service) RotateKey(newVersion string, newKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newVersion = strings.TrimSpace(newVersion)
	if newVersion == "" {
		return fmt.Errorf("new key version is required")
	}
	if len(newKey) != 32 {
		return fmt.Errorf("new key must be 32 bytes")
	}
	if newVersion == s.keyVersion {
		return nil
	}

	s.legacyKeys[s.keyVersion] = legacyKey{
		key:       append([]byte(nil), s.currentKey...),
		expiresAt: s.now().Add(s.rotationWindow),
	}
	s.currentKey = append([]byte(nil), newKey...)
	s.keyVersion = newVersion
	return nil
}

func (s *Service) EncryptField(field, plaintext string) (CipherRecord, error) {
	s.mu.RLock()
	key := append([]byte(nil), s.currentKey...)
	keyVersion := s.keyVersion
	s.mu.RUnlock()

	record, err := encryptWithKey(key, keyVersion, field, plaintext)
	if err != nil {
		return CipherRecord{}, err
	}
	return record, nil
}

func (s *Service) EncryptPayload(fields map[string]string) (map[string]CipherRecord, error) {
	s.mu.RLock()
	policies := make(map[string]FieldPolicy, len(s.fieldPolicies))
	for field, policy := range s.fieldPolicies {
		policies[field] = policy
	}
	s.mu.RUnlock()

	out := map[string]CipherRecord{}
	for field, value := range fields {
		policy, hasPolicy := policies[field]
		if hasPolicy && !policy.Required {
			continue
		}
		record, err := s.EncryptField(field, value)
		if err != nil {
			return nil, err
		}
		out[field] = record
	}
	return out, nil
}

func (s *Service) EnforceRequiredEncryption(plaintext map[string]string, encrypted map[string]CipherRecord) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for field, policy := range s.fieldPolicies {
		if !policy.Required {
			continue
		}
		if _, hasPlaintext := plaintext[field]; !hasPlaintext {
			continue
		}
		record, ok := encrypted[field]
		if !ok || strings.TrimSpace(record.Ciphertext) == "" || strings.TrimSpace(record.Nonce) == "" {
			return fmt.Errorf("PII_ENCRYPTION_REQUIRED_%s", field)
		}
	}
	return nil
}

func (s *Service) DecryptField(record CipherRecord) (string, error) {
	s.mu.RLock()
	currentKey := append([]byte(nil), s.currentKey...)
	currentVersion := s.keyVersion
	legacy, hasLegacy := s.legacyKeys[record.KeyVersion]
	now := s.now()
	s.mu.RUnlock()

	if record.KeyVersion == currentVersion || record.KeyVersion == "" {
		return decryptWithKey(currentKey, record)
	}
	if hasLegacy && now.Before(legacy.expiresAt) {
		return decryptWithKey(legacy.key, record)
	}
	return "", fmt.Errorf("KEY_VERSION_EXPIRED")
}

func (s *Service) Redact(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "***"
	}
	if strings.Contains(value, "@") {
		parts := strings.Split(value, "@")
		if len(parts) == 2 && len(parts[0]) > 1 {
			return parts[0][:1] + "***@" + parts[1]
		}
	}
	phoneDigits := regexp.MustCompile(`\d`).FindAllString(value, -1)
	if len(phoneDigits) >= 4 {
		return "***" + strings.Join(phoneDigits[len(phoneDigits)-4:], "")
	}
	if len(value) >= 4 {
		return fmt.Sprintf("***%s", value[len(value)-4:])
	}
	return "***"
}

func encryptWithKey(key []byte, keyVersion, field, plaintext string) (CipherRecord, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return CipherRecord{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return CipherRecord{}, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return CipherRecord{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return CipherRecord{
		Field:      field,
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		KeyVersion: keyVersion,
	}, nil
}

func decryptWithKey(key []byte, record CipherRecord) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
