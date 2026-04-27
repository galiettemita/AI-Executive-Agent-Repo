package identity

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func parseRSAPublicKeyPEM(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, fmt.Errorf("invalid public key pem")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not rsa")
	}
	return publicKey, nil
}

func MarshalRSAPublicKeyPEM(publicKey *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	block := pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return string(pem.EncodeToMemory(&block)), nil
}

func extractBearerToken(header string) string {
	normalized := strings.TrimSpace(header)
	if normalized == "" || !strings.HasPrefix(strings.ToLower(normalized), "bearer ") {
		return ""
	}
	return strings.TrimSpace(normalized[7:])
}

func VerifyAdminHTTPRequest(r *http.Request, expectedAudience string, now time.Time) (AdminJWTClaims, error) {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return AdminJWTClaims{}, fmt.Errorf("authorization required")
	}
	publicKeyPEM := strings.TrimSpace(os.Getenv("BREVIO_AUTH_ACCESS_PUBLIC_KEY"))
	if publicKeyPEM == "" {
		return AdminJWTClaims{}, fmt.Errorf("BREVIO_AUTH_ACCESS_PUBLIC_KEY is required")
	}
	publicKey, err := parseRSAPublicKeyPEM(publicKeyPEM)
	if err != nil {
		return AdminJWTClaims{}, err
	}
	signer := NewJWTVerifier(publicKey)
	expectedIssuer := strings.TrimSpace(os.Getenv("BREVIO_AUTH_ACCESS_ISSUER"))
	if expectedIssuer == "" {
		expectedIssuer = "https://auth.brevio.internal"
	}
	if expectedAudience == "" {
		expectedAudience = AdminJWTAudience()
	}
	return signer.VerifyAdminJWT(token, expectedIssuer, expectedAudience, now)
}
