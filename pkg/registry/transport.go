package registry

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// newHTTPTransport returns a clone of http.DefaultTransport with the given TLS config.
func newHTTPTransport(tlsConfig *tls.Config) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = tlsConfig

	return tr
}

// reloadingTransport is an http.RoundTripper backed by an *http.Transport whose RootCAs are loaded from PEM
// files on disk. Reload re-reads the files and swaps the underlying transport only when their content changed;
// Checker.Tick calls it before every round of image checks.
type reloadingTransport struct {
	caPaths []string

	current atomic.Pointer[http.Transport]
	last    [][]byte // contents of caPaths as of the last successful load
	failing bool     // whether the previous reload failed; reloadAndLog logs state transitions only
}

// newReloadingTransport loads the CA files once; a missing or unparsable file is an error (fail fast at startup).
func newReloadingTransport(caPaths []string) (*reloadingTransport, error) {
	if len(caPaths) == 0 {
		return nil, errors.New("no CA files given")
	}

	t := &reloadingTransport{caPaths: caPaths}
	if _, err := t.Reload(); err != nil {
		return nil, err
	}

	return t, nil
}

// RoundTrip implements http.RoundTripper.
func (t *reloadingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.current.Load().RoundTrip(req)
}

// Reload re-reads the CA files and replaces the underlying transport if their content changed. On error the
// previously loaded transport is kept intact. Reload must not be called concurrently with itself; it is called by
// newReloadingTransport and then only from Checker.Tick.
func (t *reloadingTransport) Reload() (changed bool, err error) {
	rootCAs, contents, err := loadCAs(t.caPaths)
	if err != nil {
		return false, err
	}
	if slices.EqualFunc(contents, t.last, bytes.Equal) {
		return false, nil
	}

	old := t.current.Swap(newHTTPTransport(&tls.Config{RootCAs: rootCAs}))
	t.last = contents
	if old != nil {
		// Pooled keep-alive connections were verified against the old pool; drop them so new ones get re-verified.
		old.CloseIdleConnections()
	}

	return true, nil
}

// reloadAndLog is Reload with logging, for use from Checker.Tick. Only state transitions are logged: the first
// failure, the recovery and an actual change, so a persistent error does not repeat on every tick.
func (t *reloadingTransport) reloadAndLog() {
	changed, err := t.Reload()
	if err != nil {
		if !t.failing {
			logrus.Errorf("Failed to reload CA certificates, keeping previously loaded ones: %v", err)
		}
		t.failing = true
		return
	}
	if changed {
		logrus.Infof("Reloaded CA certificates from %s", strings.Join(t.caPaths, ", "))
	} else if t.failing {
		logrus.Infof("CA certificates from %s are readable again, content unchanged", strings.Join(t.caPaths, ", "))
	}
	t.failing = false
}

// loadCAs reads and parses every CA file on top of the system pool. The raw file contents are returned
// alongside the pool so that the caller can detect changes.
func loadCAs(caPaths []string) (*x509.CertPool, [][]byte, error) {
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	contents := make([][]byte, 0, len(caPaths))
	for _, caPath := range caPaths {
		pemCerts, err := os.ReadFile(caPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read CA file %q: %w", caPath, err)
		}
		if !rootCAs.AppendCertsFromPEM(pemCerts) {
			return nil, nil, fmt.Errorf("parse CA file %q: no valid PEM certificates found", caPath)
		}
		contents = append(contents, pemCerts)
	}

	return rootCAs, contents, nil
}
