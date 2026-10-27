package registry

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// newTLSServer starts a TLS server presenting cert and returns it with a counter of accepted connections.
func newTLSServer(t *testing.T, cert tls.Certificate) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return srv, &conns
}

func doGet(tr http.RoundTripper, url string) (int, error) {
	resp, err := (&http.Client{Transport: tr}).Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

func TestReloadingTransport_PicksUpNewCA(t *testing.T) {
	caA, _ := generateCA(t)
	caB, certB := generateCA(t)
	srv, _ := newTLSServer(t, certB)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	writeFile(t, caPath, caA)

	tr, err := newReloadingTransport([]string{caPath})
	require.NoError(t, err)

	// The server presents a certificate from CA B, but only CA A is trusted.
	_, err = doGet(tr, srv.URL)
	var certErr *tls.CertificateVerificationError
	require.ErrorAs(t, err, &certErr)

	writeFile(t, caPath, caB)
	changed, err := tr.Reload()
	require.NoError(t, err)
	require.True(t, changed)

	status, err := doGet(tr, srv.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	changed, err = tr.Reload()
	require.NoError(t, err)
	require.False(t, changed, "unchanged file must not trigger a reload")
}

func TestReloadingTransport_ClosesIdleConnections(t *testing.T) {
	caA, _ := generateCA(t)
	caB, certB := generateCA(t)
	srv, conns := newTLSServer(t, certB)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	writeFile(t, caPath, caB)

	tr, err := newReloadingTransport([]string{caPath})
	require.NoError(t, err)

	status, err := doGet(tr, srv.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.EqualValues(t, 1, conns.Load())

	// Same server, same trust: a second request reuses the pooled keep-alive connection.
	_, err = doGet(tr, srv.URL)
	require.NoError(t, err)
	require.EqualValues(t, 1, conns.Load())

	// A changed bundle must drop pooled connections so that the next request is verified against the new pool.
	writeFile(t, caPath, append(append([]byte{}, caB...), caA...))
	changed, err := tr.Reload()
	require.NoError(t, err)
	require.True(t, changed)

	_, err = doGet(tr, srv.URL)
	require.NoError(t, err)
	require.EqualValues(t, 2, conns.Load())
}

func TestReloadingTransport_KeepsOldOnError(t *testing.T) {
	caB, certB := generateCA(t)
	srv, _ := newTLSServer(t, certB)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	writeFile(t, caPath, caB)

	tr, err := newReloadingTransport([]string{caPath})
	require.NoError(t, err)

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

		status, err := doGet(tr, srv.URL)
		require.NoError(t, err, "old transport must keep working after a failed reload (%s)", tt.name)
		require.Equal(t, http.StatusOK, status)
	}

	require.NoError(t, os.Remove(caPath))
	changed, err := tr.Reload()
	require.Error(t, err)
	require.False(t, changed)
	_, err = doGet(tr, srv.URL)
	require.NoError(t, err, "old transport must keep working when the file is missing")
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
	_, err = doGet(tr, srv.URL)
	require.Error(t, err)

	// A change in the second file alone triggers a reload.
	writeFile(t, secondPath, caB)
	changed, err := tr.Reload()
	require.NoError(t, err)
	require.True(t, changed)
	_, err = doGet(tr, srv.URL)
	require.NoError(t, err)

	// An error in the first file keeps the transport built from the previous content of both files.
	writeFile(t, firstPath, []byte("not a pem"))
	changed, err = tr.Reload()
	require.Error(t, err)
	require.False(t, changed)
	_, err = doGet(tr, srv.URL)
	require.NoError(t, err)
}

func TestNewReloadingTransport_Errors(t *testing.T) {
	_, err := newReloadingTransport([]string{filepath.Join(t.TempDir(), "missing.pem")})
	require.Error(t, err)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	writeFile(t, caPath, []byte("not a pem"))
	_, err = newReloadingTransport([]string{caPath})
	require.Error(t, err)
}
