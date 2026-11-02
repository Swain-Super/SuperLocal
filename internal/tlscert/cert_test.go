package tlscert

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreateIncludesHostsAndKeyMode(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	cert, err := LoadOrCreate(dir, []string{"192.168.1.8"}, now)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.PublicKeyAlgorithm != x509.ECDSA {
		t.Fatalf("algorithm %s", leaf.PublicKeyAlgorithm)
	}
	for _, name := range []string{"localhost", "127.0.0.1", "192.168.1.8"} {
		if err := leaf.VerifyHostname(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "tls", "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	again, err := LoadOrCreate(dir, []string{"192.168.1.8"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameCert(cert, again) {
		t.Fatal("reloaded a different certificate")
	}
}

func TestLoadOrCreateRegeneratesWhenAddressSetChanges(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	first, err := LoadOrCreate(dir, []string{"192.168.1.8"}, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreate(dir, []string{"192.168.1.9"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if sameCert(first, second) {
		t.Fatal("address change reused the old certificate")
	}
	leaf, err := x509.ParseCertificate(second.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := leaf.VerifyHostname("192.168.1.9"); err != nil {
		t.Fatal(err)
	}
}

func sameCert(a, b tls.Certificate) bool {
	if len(a.Certificate) == 0 || len(b.Certificate) == 0 {
		return false
	}
	return string(a.Certificate[0]) == string(b.Certificate[0])
}
