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

// reloadingTransport is an http.RoundTripper backed by an *http.Transport whose RootCAs are loaded from PEM
// files on disk. Reload re-reads the files and swaps the underlying transport only when their content changed.
type reloadingTransport struct {
	caPaths []string

	current atomic.Pointer[http.Transport]

	// Reload is called from a single goroutine in production; the mutex makes direct concurrent calls (tests) safe.
	reloadMu sync.Mutex
	last     [][]byte // contents of caPaths as of the last successful load, guarded by reloadMu
}

func newReloadingTransport(caPaths []string) (*reloadingTransport, error) {
	contents, err := readCAFiles(caPaths)
	if err != nil {
		return nil, err
	}

	tr, err := newCATransport(caPaths, contents)
	if err != nil {
		return nil, err
	}

	t := &reloadingTransport{caPaths: caPaths, last: contents}
	t.current.Store(tr)

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

	contents, err := readCAFiles(t.caPaths)
	if err != nil {
		return false, err
	}
	if slices.EqualFunc(contents, t.last, bytes.Equal) {
		return false, nil
	}

	tr, err := newCATransport(t.caPaths, contents)
	if err != nil {
		return false, err
	}

	old := t.current.Swap(tr)
	t.last = contents
	// Pooled keep-alive connections were verified against the old pool; drop them so new ones get re-verified.
	old.CloseIdleConnections()

	return true, nil
}

// Run re-reads the CA files every interval until stopCh is closed. wait.Until runs the function immediately;
// that first pass is a no-op since the files were just loaded by newReloadingTransport.
func (t *reloadingTransport) Run(interval time.Duration, stopCh <-chan struct{}) {
	wait.Until(func() {
		changed, err := t.Reload()
		if err != nil {
			logrus.Errorf("Failed to reload CA certificates, keeping previously loaded ones: %v", err)
			return
		}
		if changed {
			logrus.Infof("Reloaded CA certificates from %s", strings.Join(t.caPaths, ", "))
		}
	}, interval, stopCh)
}

func readCAFiles(caPaths []string) ([][]byte, error) {
	contents := make([][]byte, 0, len(caPaths))
	for _, caPath := range caPaths {
		pemCerts, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read CA file %q: %w", caPath, err)
		}
		contents = append(contents, pemCerts)
	}

	return contents, nil
}

func newCATransport(caPaths []string, contents [][]byte) (*http.Transport, error) {
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	for i, pemCerts := range contents {
		if ok := rootCAs.AppendCertsFromPEM(pemCerts); !ok {
			return nil, fmt.Errorf("parse CA file %q: no PEM certificates found", caPaths[i])
		}
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{RootCAs: rootCAs}

	return tr, nil
}
