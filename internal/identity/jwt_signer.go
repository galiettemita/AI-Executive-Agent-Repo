package identity

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type JWTSigner struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string
}

func NewJWTSigner(privateKey *rsa.PrivateKey) *JWTSigner {
	keyID := ""
	var publicKey *rsa.PublicKey
	if privateKey != nil {
		sum := sha256.Sum256(privateKey.PublicKey.N.Bytes())
		keyID = base64.RawURLEncoding.EncodeToString(sum[:8])
		publicKey = &privateKey.PublicKey
	}
	return &JWTSigner{
		privateKey: privateKey,
		publicKey:  publicKey,
		keyID:      keyID,
	}
}

func GenerateJWTSigningKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

func (s *JWTSigner) IssueUserJWT(claims UserJWTClaims, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if claims.Iss == "" {
		claims.Iss = "https://auth.brevio.app"
	}
	if claims.Aud == "" {
		claims.Aud = "https://api.brevio.local"
	}
	if claims.Iat == 0 {
		claims.Iat = now.Unix()
	}
	if claims.Exp == 0 {
		claims.Exp = now.Add(UserJWTLifetime()).Unix()
	}
	return s.issueToken(claims)
}

func (s *JWTSigner) IssueAdminJWT(claims AdminJWTClaims, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if claims.Iss == "" {
		claims.Iss = "https://auth.brevio.app"
	}
	if claims.Aud == "" {
		claims.Aud = "https://api.brevio.local"
	}
	if claims.Iat == 0 {
		claims.Iat = now.Unix()
	}
	if claims.Exp == 0 {
		claims.Exp = now.Add(AdminJWTLifetime()).Unix()
	}
	return s.issueToken(claims)
}

func (s *JWTSigner) issueToken(payload any) (string, error) {
	if s == nil || s.privateKey == nil {
		return "", fmt.Errorf("jwt signer private key is required")
	}
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": s.keyID,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := headerB64 + "." + payloadB64
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (s *JWTSigner) VerifyUserJWT(token, expectedIssuer, expectedAudience string, now time.Time) (UserJWTClaims, error) {
	payload, err := s.verifyTokenSignature(token)
	if err != nil {
		return UserJWTClaims{}, err
	}
	var claims UserJWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return UserJWTClaims{}, err
	}
	if err := validateClaims(claims.Iss, claims.Aud, claims.Exp, expectedIssuer, expectedAudience, now); err != nil {
		return UserJWTClaims{}, err
	}
	return claims, nil
}

func (s *JWTSigner) VerifyAdminJWT(token, expectedIssuer, expectedAudience string, now time.Time) (AdminJWTClaims, error) {
	payload, err := s.verifyTokenSignature(token)
	if err != nil {
		return AdminJWTClaims{}, err
	}
	var claims AdminJWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return AdminJWTClaims{}, err
	}
	if err := validateClaims(claims.Iss, claims.Aud, claims.Exp, expectedIssuer, expectedAudience, now); err != nil {
		return AdminJWTClaims{}, err
	}
	return claims, nil
}

func (s *JWTSigner) verifyTokenSignature(token string) ([]byte, error) {
	if s == nil || s.publicKey == nil {
		return nil, fmt.Errorf("jwt signer public key is required")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid jwt format")
	}
	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid jwt signature encoding")
	}
	sum := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(s.publicKey, crypto.SHA256, sum[:], signature); err != nil {
		return nil, fmt.Errorf("invalid jwt signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid jwt payload encoding")
	}
	return payload, nil
}

func validateClaims(issuer, audience string, exp int64, expectedIssuer, expectedAudience string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if expectedIssuer != "" && issuer != expectedIssuer {
		return fmt.Errorf("issuer mismatch")
	}
	if expectedAudience != "" && audience != expectedAudience {
		return fmt.Errorf("audience mismatch")
	}
	if exp > 0 && now.UTC().Unix() > exp {
		return fmt.Errorf("token expired")
	}
	return nil
}

func (s *JWTSigner) JWKS() map[string]any {
	if s == nil || s.publicKey == nil {
		return map[string]any{"keys": []map[string]string{}}
	}
	return map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": s.keyID,
				"n":   base64.RawURLEncoding.EncodeToString(s.publicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(s.publicKey.E)).Bytes()),
			},
		},
	}
}
