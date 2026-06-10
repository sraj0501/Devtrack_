package trigger

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// localSANs returns the SANs for the self-signed cert: loopback addresses,
// this machine's hostname, and every non-loopback, non-link-local IP bound to a
// local network interface (so the cert is valid via localhost or a LAN IP).
func localSANs() ([]net.IP, []string) {
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	dns := []string{"localhost"}

	if host, err := os.Hostname(); err == nil && host != "" && host != "localhost" {
		dns = append(dns, host)
	}

	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
				continue
			}
			ips = append(ips, ipnet.IP)
		}
	}
	return ips, dns
}

// GenerateSelfSignedCert creates an ECDSA P-256 self-signed TLS certificate
// valid for 1 year.  Both files are written with mode 0600.
// Existing files are overwritten (cert is regenerated on every daemon start).
//
// SANs cover loopback (127.0.0.1, ::1, localhost) plus this machine's hostname
// and every non-loopback address bound to a local interface, so the cert is
// valid whether the server is reached via localhost or its LAN IP/hostname.
func GenerateSelfSignedCert(certPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	ips, dnsNames := localSANs()

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{"DevTrack"}},
		NotBefore:             time.Now().Add(-time.Minute), // slight back-date avoids clock-skew
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           ips,
		DNSNames:              dnsNames,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0700); err != nil {
		return fmt.Errorf("create tls dir: %w", err)
	}

	if err := writePEMFile(certPath, "CERTIFICATE", certDER); err != nil {
		return err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	return writePEMFile(keyPath, "EC PRIVATE KEY", keyDER)
}

func writePEMFile(path, pemType string, der []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: pemType, Bytes: der})
}

// CertExistsAndValid returns true if certPath exists, parses as a valid PEM
// certificate, and has at least minTTLDays days left before expiry.
// Returns false (not an error) when the cert is missing, corrupt, or stale —
// callers should regenerate in that case.
func CertExistsAndValid(certPath string, minTTLDays int) bool {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	return time.Until(cert.NotAfter) >= time.Duration(minTTLDays)*24*time.Hour
}

// LoadTLSCertPool reads a PEM certificate file and returns a *x509.CertPool
// that trusts exactly that certificate (cert-pinning for self-signed certs).
func LoadTLSCertPool(certPath string) (*x509.CertPool, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read cert %s: %w", certPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("no valid PEM certificate found in %s", certPath)
	}
	return pool, nil
}
