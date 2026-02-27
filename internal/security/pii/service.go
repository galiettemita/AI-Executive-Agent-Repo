package pii

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

type CipherRecord struct {
	Field      string `json:"field"`
	Ciphertext string `json:"ciphertext"`
	Nonce      string `json:"nonce"`
	KeyVersion string `json:"key_version"`
}

type Service struct {
	key        []byte
	keyVersion string
}

func NewService() *Service {
	return &Service{
		key:        []byte("0123456789abcdef0123456789abcdef"),
		keyVersion: "v1",
	}
}

func (s *Service) EncryptField(field, plaintext string) (CipherRecord, error) {
	block, err := aes.NewCipher(s.key)
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
		KeyVersion: s.keyVersion,
	}, nil
}

func (s *Service) DecryptField(record CipherRecord) (string, error) {
	block, err := aes.NewCipher(s.key)
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

func (s *Service) Redact(value string) string {
	if strings.Contains(value, "@") {
		parts := strings.Split(value, "@")
		if len(parts) == 2 && len(parts[0]) > 1 {
			return parts[0][:1] + "***@" + parts[1]
		}
	}
	if len(value) >= 4 {
		return fmt.Sprintf("***%s", value[len(value)-4:])
	}
	return "***"
}
