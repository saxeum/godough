// Command tlscheck inspects the Postgres TLS material the application trusts.
//
// It reads DATABASE_URL, checks every certificate in sslrootcert, connects with
// the same pgx TLS stack the API uses, and checks the server's peer
// certificate. It exits non-zero when anything expires inside the warning
// window, so a CronJob can alert before the RDS rotation deadline rather than
// after the first failed connection.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	minDays := flag.Int("min-days", 30, "fail when a certificate expires within this many days")
	flag.Parse()

	if err := run(*minDays); err != nil {
		fmt.Fprintf(os.Stderr, "tls-check: %v\n", err)
		os.Exit(1)
	}
}

// connConfig parses the DSN with the pool's parser, not the connection's. The
// deployment DSN carries pool_max_conns so the connection budget is not a guess,
// and pgx treats an unknown keyword as a server runtime parameter. Against RDS
// that is a hard failure — `FATAL: unrecognized configuration parameter
// "pool_max_conns"` — so a check that passes from a laptop with a bare DSN does
// not work where it matters.
func connConfig(dsn string) (*pgx.ConnConfig, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	return cfg.ConnConfig, nil
}

func run(minDays int) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is not set")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	mode := u.Query().Get("sslmode")
	if mode == "" {
		mode = "prefer"
	}
	if mode == "disable" {
		fmt.Println("TLS disabled (sslmode=disable); nothing to check")
		return nil
	}
	if mode != "verify-full" {
		fmt.Printf("warning: sslmode=%s; production expects verify-full\n", mode)
	}

	var failures []error
	if caFile := u.Query().Get("sslrootcert"); caFile != "" {
		certs, err := readCertFile(caFile)
		if err != nil {
			return err
		}
		for i, cert := range certs {
			if err := report(fmt.Sprintf("CA[%d] %s", i, caFile), cert, minDays); err != nil {
				failures = append(failures, err)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := connConfig(dsn)
	if err != nil {
		return err
	}

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", u.Host, err)
	}
	defer conn.Close(context.Background())

	tlsConn, ok := conn.PgConn().Conn().(*tls.Conn)
	if !ok {
		return fmt.Errorf("connection to %s is not TLS (sslmode=%s)", u.Host, mode)
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return errors.New("TLS connection presented no peer certificate")
	}
	for i, cert := range state.PeerCertificates {
		if err := report(fmt.Sprintf("server[%d] %s", i, u.Host), cert, minDays); err != nil {
			failures = append(failures, err)
		}
	}

	return errors.Join(failures...)
}

func readCertFile(path string) ([]*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sslrootcert %s: %w", path, err)
	}

	var certs []*x509.Certificate
	for len(raw) > 0 {
		var block *pem.Block
		block, raw = pem.Decode(raw)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate in %s: %w", path, err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates in sslrootcert %s", path)
	}
	return certs, nil
}

func report(label string, cert *x509.Certificate, minDays int) error {
	days := int(time.Until(cert.NotAfter).Hours() / 24)
	fmt.Printf("%s: subject=%q notAfter=%s (%d days)\n", label, cert.Subject.String(), cert.NotAfter.UTC().Format(time.RFC3339), days)

	if time.Now().Add(time.Duration(minDays) * 24 * time.Hour).After(cert.NotAfter) {
		return fmt.Errorf("%s expires %s, inside the %d-day window", label, cert.NotAfter.UTC().Format(time.RFC3339), minDays)
	}
	return nil
}
