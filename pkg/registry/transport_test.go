package registry

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// generateCA returns a self-signed CA in PEM form together with a tls.Certificate that a test server can
// present directly (the CA cert itself carries the loopback SANs).
func generateCA(t *testing.T) ([]byte, tls.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "k8s-image-availability-exporter test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	return certPEM, tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// connCounter counts connections accepted and closed by a test server.
type connCounter struct {
	opened atomic.Int32
	closed atomic.Int32
}

// newTLSServer starts a TLS server presenting cert. Expected handshake failures are not logged.
func newTLSServer(t *testing.T, cert tls.Certificate) (*httptest.Server, *connCounter) {
	t.Helper()

	conns := &connCounter{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			conns.opened.Add(1)
		case http.StateClosed:
			conns.closed.Add(1)
		}
	}
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return srv, conns
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

// newTestTransport writes content into a temporary CA file and builds a reloadingTransport from it.
func newTestTransport(t *testing.T, content []byte) (*reloadingTransport, string) {
	t.Helper()

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	writeFile(t, caPath, content)
	tr, err := newReloadingTransport([]string{caPath})
	require.NoError(t, err)

	return tr, caPath
}

func doGet(tr http.RoundTripper, url string) (int, error) {
	resp, err := (&http.Client{Transport: tr}).Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func requireGetOK(t *testing.T, tr http.RoundTripper, url string) {
	t.Helper()

	status, err := doGet(tr, url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
}

func requireGetUntrusted(t *testing.T, tr http.RoundTripper, url string) {
	t.Helper()

	_, err := doGet(tr, url)
	var certErr *tls.CertificateVerificationError
	require.ErrorAs(t, err, &certErr)
}

func TestReloadingTransport_PicksUpNewCA(t *testing.T) {
	caA, _ := generateCA(t)
	caB, certB := generateCA(t)
	srv, _ := newTLSServer(t, certB)

	// The server presents a certificate from CA B, but only CA A is trusted.
	tr, caPath := newTestTransport(t, caA)
	requireGetUntrusted(t, tr, srv.URL)

	// Bundles may carry comments before and after blocks, as system bundles do.
	writeFile(t, caPath, slices.Concat([]byte("# CA B\n"), caB, []byte("# end of bundle\n")))
	changed, err := tr.Reload()
	require.NoError(t, err)
	require.True(t, changed)
	requireGetOK(t, tr, srv.URL)

	changed, err = tr.Reload()
	require.NoError(t, err)
	require.False(t, changed, "unchanged file must not trigger a reload")
}

func TestReloadingTransport_ClosesIdleConnections(t *testing.T) {
	caA, _ := generateCA(t)
	caB, certB := generateCA(t)
	srv, conns := newTLSServer(t, certB)
	tr, caPath := newTestTransport(t, caB)

	// Two requests over one pooled keep-alive connection.
	requireGetOK(t, tr, srv.URL)
	requireGetOK(t, tr, srv.URL)
	require.EqualValues(t, 1, conns.opened.Load())
	require.EqualValues(t, 0, conns.closed.Load())

	// A changed bundle must drop the pooled connection: the server sees it closed without any new request.
	writeFile(t, caPath, slices.Concat(caB, caA))
	changed, err := tr.Reload()
	require.NoError(t, err)
	require.True(t, changed)
	require.Eventually(t, func() bool { return conns.closed.Load() == 1 }, 5*time.Second, 10*time.Millisecond)

	requireGetOK(t, tr, srv.URL)
	require.EqualValues(t, 2, conns.opened.Load())
}

func TestReloadingTransport_KeepsOldOnError(t *testing.T) {
	caB, certB := generateCA(t)
	srv, _ := newTLSServer(t, certB)
	tr, caPath := newTestTransport(t, caB)

	for _, tt := range []struct {
		name    string
		content []byte
	}{
		{name: "garbage", content: []byte("not a pem")},
		{name: "non-cert PEM", content: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2, 3}})},
		{name: "empty", content: []byte{}},
	} {
		writeFile(t, caPath, tt.content)
		changed, err := tr.Reload()
		require.Error(t, err, tt.name)
		require.False(t, changed, tt.name)
		requireGetOK(t, tr, srv.URL)
	}

	require.NoError(t, os.Remove(caPath))
	changed, err := tr.Reload()
	require.Error(t, err)
	require.False(t, changed)
	requireGetOK(t, tr, srv.URL)

	// Failed reloads must not touch the remembered content: restoring the original file is not a change.
	writeFile(t, caPath, caB)
	changed, err = tr.Reload()
	require.NoError(t, err)
	require.False(t, changed)
}

func TestReloadingTransport_MultipleFiles(t *testing.T) {
	caA, _ := generateCA(t)
	caB, certB := generateCA(t)
	srv, _ := newTLSServer(t, certB)

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.pem")
	secondPath := filepath.Join(dir, "second.pem")
	writeFile(t, firstPath, caA)
	writeFile(t, secondPath, caA)

	tr, err := newReloadingTransport([]string{firstPath, secondPath})
	require.NoError(t, err)
	requireGetUntrusted(t, tr, srv.URL)

	// A change in the second file alone triggers a reload.
	writeFile(t, secondPath, caB)
	changed, err := tr.Reload()
	require.NoError(t, err)
	require.True(t, changed)
	requireGetOK(t, tr, srv.URL)

	// An error in the first file keeps the previously built transport.
	writeFile(t, firstPath, []byte("not a pem"))
	changed, err = tr.Reload()
	require.Error(t, err)
	require.False(t, changed)
	requireGetOK(t, tr, srv.URL)
}

func TestReloadingTransport_Run(t *testing.T) {
	caA, _ := generateCA(t)
	caB, certB := generateCA(t)
	srv, _ := newTLSServer(t, certB)
	tr, caPath := newTestTransport(t, caA)
	requireGetUntrusted(t, tr, srv.URL)

	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		tr.Run(10*time.Millisecond, stopCh)
	}()
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { close(stopCh) }) }
	t.Cleanup(stop) // also stops Run if an assertion below fails

	writeFile(t, caPath, caB)
	require.Eventually(t, func() bool {
		status, err := doGet(tr, srv.URL)
		return err == nil && status == http.StatusOK
	}, 5*time.Second, 10*time.Millisecond, "Run must pick up the new CA")

	stop()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after stopCh was closed")
	}
}

func TestNewReloadingTransport_Errors(t *testing.T) {
	_, err := newReloadingTransport(nil)
	require.Error(t, err, "empty path list must be rejected")

	_, err = newReloadingTransport([]string{filepath.Join(t.TempDir(), "missing.pem")})
	require.Error(t, err)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	writeFile(t, caPath, []byte("not a pem"))
	_, err = newReloadingTransport([]string{caPath})
	require.Error(t, err)
}
