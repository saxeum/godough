package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"time"
)

func testCert(t *testing.T, notAfter time.Time) *x509.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tlscheck-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func TestReportFailsInsideTheWindow(t *testing.T) {
	cert := testCert(t, time.Now().Add(5*24*time.Hour))
	if err := report("short", cert, 30); err == nil {
		t.Fatal("report accepted a certificate inside the 30-day window")
	}
}

func TestReportPassesOutsideTheWindow(t *testing.T) {
	cert := testCert(t, time.Now().Add(90*24*time.Hour))
	if err := report("long", cert, 30); err != nil {
		t.Fatalf("report rejected a certificate with 90 days left: %v", err)
	}
}

func TestReadCertFileReportsAnEmptyBundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(path, []byte("not a certificate\n"), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	if _, err := readCertFile(path); err == nil {
		t.Fatal("readCertFile accepted a bundle with no certificates")
	}
}

func TestReadCertFileReadsOneCertificate(t *testing.T) {
	cert := testCert(t, time.Now().Add(90*24*time.Hour))
	path := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	certs, err := readCertFile(path)
	if err != nil {
		t.Fatalf("readCertFile: %v", err)
	}
	if len(certs) != 1 || certs[0].Subject.CommonName != "tlscheck-test" {
		t.Fatalf("certs = %+v", certs)
	}
}

// The deployment DSN carries pool_max_conns, and pgx sends an unknown keyword to
// the server as a runtime parameter. Against RDS that is fatal, so this has to be
// the pool's parser. The negative control is the connection parser, which does
// forward it: that is the bug this function exists to avoid.
func TestConnConfigDoesNotSendPoolParametersToTheServer(t *testing.T) {
	dsn := "postgres://user:pass@example.com:5432/db?sslmode=verify-full&pool_max_conns=5&pool_max_conn_lifetime=30m"

	cfg, err := connConfig(dsn)
	if err != nil {
		t.Fatalf("connConfig: %v", err)
	}
	for _, key := range []string{"pool_max_conns", "pool_max_conn_lifetime"} {
		if _, sent := cfg.RuntimeParams[key]; sent {
			t.Errorf("%s would be sent to the server as a runtime parameter", key)
		}
	}

	connCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig: %v", err)
	}
	if _, sent := connCfg.RuntimeParams["pool_max_conns"]; !sent {
		t.Skip("pgx no longer forwards unknown keywords; this comment can be simplified")
	}
}
