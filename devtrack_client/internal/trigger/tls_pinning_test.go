package trigger

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestSelfSignedPinning reproduces the same-machine TLS path: generate the
// daemon's self-signed cert, serve TLS with it on 127.0.0.1, then connect with
// a client that cert-pins it (RootCAs = the self-signed cert), exactly as
// NewHTTPTriggerClient does. If this fails, same-machine connections fail.
func TestSelfSignedPinning(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	if err := GenerateSelfSignedCert(certPath, keyPath); err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}

	serverCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("load keypair: %v", err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
	srv.StartTLS()
	defer srv.Close()

	// Client cert-pins the self-signed cert (same as NewHTTPTriggerClient).
	pool, err := LoadTLSCertPool(certPath)
	if err != nil {
		t.Fatalf("LoadTLSCertPool: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	}

	resp, err := client.Get(srv.URL) // srv.URL is https://127.0.0.1:<port>
	if err != nil {
		t.Fatalf("PINNING FAILED (same-machine path is broken): %v", err)
	}
	resp.Body.Close()
	t.Logf("pinning OK against %s", srv.URL)

	// Also check what SANs the cert actually carries.
	leaf := serverCert.Leaf
	if leaf == nil {
		leaf, _ = x509.ParseCertificate(serverCert.Certificate[0])
	}
	t.Logf("cert IPs=%v DNS=%v IsCA=%v BasicConstraintsValid=%v KeyUsage=%d",
		leaf.IPAddresses, leaf.DNSNames, leaf.IsCA, leaf.BasicConstraintsValid, leaf.KeyUsage)
}
