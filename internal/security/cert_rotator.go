package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// SelfSignedCertRotator generates and rotates self-signed mTLS certificates.
// In production, replace with SPIFFE/SPIRE or AWS ACM Private CA.
type SelfSignedCertRotator struct {
	dir         string
	serviceName string
	validFor    time.Duration
}

// NewSelfSignedCertRotator creates a rotator that writes certs to dir.
func NewSelfSignedCertRotator(dir, serviceName string, validFor time.Duration) *SelfSignedCertRotator {
	if validFor <= 0 {
		validFor = 30 * 24 * time.Hour
	}
	return &SelfSignedCertRotator{dir: dir, serviceName: serviceName, validFor: validFor}
}

func (r *SelfSignedCertRotator) certPath() string  { return filepath.Join(r.dir, r.serviceName+".crt") }
func (r *SelfSignedCertRotator) keyPath() string   { return filepath.Join(r.dir, r.serviceName+".key") }
func (r *SelfSignedCertRotator) caPath() string    { return filepath.Join(r.dir, "ca.crt") }
func (r *SelfSignedCertRotator) caKeyPath() string { return filepath.Join(r.dir, "ca.key") }

// EnsureCerts generates certs if they don't exist, or rotates if expiring soon.
func (r *SelfSignedCertRotator) EnsureCerts() error {
	if err := os.MkdirAll(r.dir, 0700); err != nil {
		return fmt.Errorf("cert rotator: mkdir %s: %w", r.dir, err)
	}
	if _, err := os.Stat(r.certPath()); err == nil {
		if fresh, _ := r.isFresh(r.certPath()); fresh {
			return nil
		}
	}
	return r.generate()
}

// RotateIfExpiringSoon regenerates the leaf cert if it expires within 7 days.
func (r *SelfSignedCertRotator) RotateIfExpiringSoon() error {
	if _, err := os.Stat(r.certPath()); os.IsNotExist(err) {
		return r.generate()
	}
	fresh, err := r.isFresh(r.certPath())
	if err != nil || !fresh {
		return r.generate()
	}
	return nil
}

func (r *SelfSignedCertRotator) isFresh(certFile string) (bool, error) {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return false, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false, fmt.Errorf("no PEM block in %s", certFile)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, err
	}
	return time.Until(cert.NotAfter) > 7*24*time.Hour, nil
}

func (r *SelfSignedCertRotator) generate() error {
	caKey, caCert, err := r.ensureCA()
	if err != nil {
		return fmt.Errorf("cert rotator: ca: %w", err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("cert rotator: leaf key: %w", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	leafTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: r.serviceName, Organization: []string{"brevio"}},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(r.validFor),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{r.serviceName, "localhost"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("cert rotator: sign leaf: %w", err)
	}

	if err := writePEM(r.certPath(), "CERTIFICATE", leafDER); err != nil {
		return err
	}
	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		return err
	}
	return writePEM(r.keyPath(), "PRIVATE KEY", leafKeyDER)
}

func (r *SelfSignedCertRotator) ensureCA() (*rsa.PrivateKey, *x509.Certificate, error) {
	if _, err := os.Stat(r.caPath()); err == nil {
		caCert, caKey, err := loadCert(r.caPath(), r.caKeyPath())
		if err == nil {
			return caKey, caCert, nil
		}
	}
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	caTemplate := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "brevio-internal-ca", Organization: []string{"brevio"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	if err := writePEM(r.caPath(), "CERTIFICATE", caDER); err != nil {
		return nil, nil, err
	}
	caKeyDER, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		return nil, nil, err
	}
	if err := writePEM(r.caKeyPath(), "PRIVATE KEY", caKeyDER); err != nil {
		return nil, nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	return caKey, caCert, err
}

// MTLSConfigForService returns an MTLSConfig for the rotated service certs.
func (r *SelfSignedCertRotator) MTLSConfigForService() *MTLSConfig {
	return &MTLSConfig{
		CertFile: r.certPath(),
		KeyFile:  r.keyPath(),
		CAFile:   r.caPath(),
	}
}

func writePEM(path, pemType string, der []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("writePEM %s: %w", path, err)
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: pemType, Bytes: der})
}

func loadCert(certFile, keyFile string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, err
	}
	certBlock, _ := pem.Decode(certPEM)
	keyBlock, _ := pem.Decode(keyPEM)
	if certBlock == nil || keyBlock == nil {
		return nil, nil, fmt.Errorf("invalid PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("not RSA key")
	}
	return cert, rsaKey, nil
}
