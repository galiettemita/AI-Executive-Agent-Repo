package security_test

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/security"
)

func TestSelfSignedCertRotator_GeneratesCerts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rotator := security.NewSelfSignedCertRotator(dir, "brain", 24*time.Hour)
	if err := rotator.EnsureCerts(); err != nil {
		t.Fatalf("EnsureCerts: %v", err)
	}
	for _, f := range []string{"brain.crt", "brain.key", "ca.crt"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing file %s: %v", f, err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dir, "brain.crt"))
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("no PEM block in brain.crt")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		t.Errorf("cert not parseable: %v", err)
	}
}

func TestSelfSignedCertRotator_SkipsIfFresh(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rotator := security.NewSelfSignedCertRotator(dir, "gateway", 30*24*time.Hour)
	if err := rotator.EnsureCerts(); err != nil {
		t.Fatalf("EnsureCerts: %v", err)
	}
	fi1, _ := os.Stat(filepath.Join(dir, "gateway.crt"))
	time.Sleep(10 * time.Millisecond)
	if err := rotator.RotateIfExpiringSoon(); err != nil {
		t.Fatalf("RotateIfExpiringSoon: %v", err)
	}
	fi2, _ := os.Stat(filepath.Join(dir, "gateway.crt"))
	if fi1.ModTime() != fi2.ModTime() {
		t.Error("cert was unnecessarily regenerated")
	}
}

func TestSelfSignedCertRotator_MTLSConfig_LoadsWithTLS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rotator := security.NewSelfSignedCertRotator(dir, "executor", 24*time.Hour)
	if err := rotator.EnsureCerts(); err != nil {
		t.Fatalf("EnsureCerts: %v", err)
	}
	cfg := rotator.MTLSConfigForService()
	if cfg == nil {
		t.Fatal("nil MTLSConfig")
	}
	tlsCfg, err := cfg.ServerTLSConfig()
	if err != nil {
		t.Fatalf("ServerTLSConfig: %v", err)
	}
	if len(tlsCfg.Certificates) == 0 {
		t.Error("no certificates in tls config")
	}
}
