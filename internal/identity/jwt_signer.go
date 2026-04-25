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

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
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
		claims.Aud = UserJWTAudience()
	}
	if claims.Iat == 0 {
		claims.Iat = now.Unix()
	}
	if claims.Exp == 0 {
		claims.Exp = now.Add(UserJWTLifetime()).Unix()
	}
	if claims.Version == 0 {
		claims.Version = 2
	}
	if claims.TokenUse == "" {
		claims.TokenUse = "user_access"
	}
	return s.issueToken(claims, "brevio-user+jwt")
}

func (s *JWTSigner) IssueAdminJWT(claims AdminJWTClaims, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if claims.Iss == "" {
		claims.Iss = "https://auth.brevio.app"
	}
	if claims.Aud == "" {
		claims.Aud = AdminJWTAudience()
	}
	if claims.Iat == 0 {
		claims.Iat = now.Unix()
	}
	if claims.Exp == 0 {
		claims.Exp = now.Add(AdminJWTLifetime()).Unix()
	}
	if claims.Version == 0 {
		claims.Version = 2
	}
	if claims.TokenUse == "" {
		claims.TokenUse = "admin_access"
	}
	return s.issueToken(claims, "brevio-admin+jwt")
}

func (s *JWTSigner) issueToken(payload any, typ string) (string, error) {
	if s == nil || s.privateKey == nil {
		return "", fmt.Errorf("jwt signer private key is required")
	}
	header := map[string]string{
		"alg": "RS256",
		"typ": typ,
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
	header, payload, err := s.verifyTokenSignature(token)
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
	if claims.Version >= 2 {
		if header.Typ != "brevio-user+jwt" {
			return UserJWTClaims{}, fmt.Errorf("token type mismatch")
		}
		if claims.TokenUse != "user_access" {
			return UserJWTClaims{}, fmt.Errorf("token_use mismatch")
		}
	}
	return claims, nil
}

func (s *JWTSigner) VerifyAdminJWT(token, expectedIssuer, expectedAudience string, now time.Time) (AdminJWTClaims, error) {
	header, payload, err := s.verifyTokenSignature(token)
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
	if claims.Version >= 2 {
		if header.Typ != "brevio-admin+jwt" {
			return AdminJWTClaims{}, fmt.Errorf("token type mismatch")
		}
		if claims.TokenUse != "admin_access" {
			return AdminJWTClaims{}, fmt.Errorf("token_use mismatch")
		}
		if claims.AdminLevel == "" || len(claims.AdminScopes) == 0 {
			return AdminJWTClaims{}, fmt.Errorf("admin claims missing")
		}
	}
	return claims, nil
}

func (s *JWTSigner) verifyTokenSignature(token string) (jwtHeader, []byte, error) {
	if s == nil || s.publicKey == nil {
		return jwtHeader{}, nil, fmt.Errorf("jwt signer public key is required")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return jwtHeader{}, nil, fmt.Errorf("invalid jwt format")
	}
	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, nil, fmt.Errorf("invalid jwt signature encoding")
	}
	sum := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(s.publicKey, crypto.SHA256, sum[:], signature); err != nil {
		return jwtHeader{}, nil, fmt.Errorf("invalid jwt signature")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, nil, fmt.Errorf("invalid jwt header encoding")
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return jwtHeader{}, nil, fmt.Errorf("invalid jwt header")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, nil, fmt.Errorf("invalid jwt payload encoding")
	}
	return header, payload, nil
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
