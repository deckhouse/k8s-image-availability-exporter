package registry

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

// newHTTPTransport returns a clone of http.DefaultTransport with the given TLS config.
func newHTTPTransport(tlsConfig *tls.Config) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = tlsConfig

	return tr
}

// reloadingTransport is an http.RoundTripper backed by an *http.Transport whose RootCAs are loaded from PEM
// files on disk. Reload re-reads the files and swaps the underlying transport only when their content changed.
type reloadingTransport struct {
	caPaths []string

	current atomic.Pointer[http.Transport]

	// Reload is called from a single goroutine in production; the mutex makes direct concurrent calls (tests) safe.
	reloadMu sync.Mutex
	last     [][]byte // contents of caPaths as of the last successful load, guarded by reloadMu
}

// newReloadingTransport loads the CA files once; a missing or unparsable file is an error (fail fast at startup).
func newReloadingTransport(caPaths []string) (*reloadingTransport, error) {
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
// previously loaded transport is kept intact.
func (t *reloadingTransport) Reload() (changed bool, err error) {
	t.reloadMu.Lock()
	defer t.reloadMu.Unlock()

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

// Run re-reads the CA files every interval until stopCh is closed.
func (t *reloadingTransport) Run(interval time.Duration, stopCh <-chan struct{}) {
	wait.Until(t.reloadAndLog, interval, stopCh)
}

// reloadAndLog is the periodic body of Run. wait.Until calls it immediately; that first pass is a no-op since
// the files were just loaded by newReloadingTransport.
func (t *reloadingTransport) reloadAndLog() {
	changed, err := t.Reload()
	if err != nil {
		logrus.Errorf("Failed to reload CA certificates, keeping previously loaded ones: %v", err)
		return
	}
	if changed {
		logrus.Infof("Reloaded CA certificates from %s", strings.Join(t.caPaths, ", "))
	}
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
			return nil, nil, fmt.Errorf("parse CA file %q: no PEM certificates found", caPath)
		}
		contents = append(contents, pemCerts)
	}

	return rootCAs, contents, nil
}
