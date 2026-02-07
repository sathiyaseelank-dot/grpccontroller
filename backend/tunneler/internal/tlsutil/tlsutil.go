package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

// CertStore keeps the current workload certificate in memory for rotation.
type CertStore struct {
	mu       sync.RWMutex
	cert     tls.Certificate
	certPEM  []byte
	notAfter time.Time
}

// NewCertStore initializes a new CertStore.
func NewCertStore(cert tls.Certificate, certPEM []byte, notAfter time.Time) *CertStore {
	return &CertStore{cert: cert, certPEM: certPEM, notAfter: notAfter}
}

// Update replaces the certificate in memory.
func (s *CertStore) Update(cert tls.Certificate, certPEM []byte, notAfter time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cert = cert
	s.certPEM = certPEM
	s.notAfter = notAfter
}

// NotAfter returns the current certificate expiry.
func (s *CertStore) NotAfter() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.notAfter
}

// GetClientCertificate returns the current certificate for client-side handshakes.
func (s *CertStore) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &s.cert, nil
}

// RootPoolFromPEM builds a cert pool from PEM bytes.
func RootPoolFromPEM(pemBytes []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, errors.New("failed to parse root CA PEM")
	}
	return pool, nil
}

// VerifyPeerSPIFFE verifies the peer chain and SPIFFE identity.
func VerifyPeerSPIFFE(rawCerts [][]byte, roots *x509.CertPool, trustDomain, expectedRole string, usage x509.ExtKeyUsage) error {
	if len(rawCerts) == 0 {
		return errors.New("no peer certificates")
	}

	certs := make([]*x509.Certificate, 0, len(rawCerts))
	for _, raw := range rawCerts {
		c, err := x509.ParseCertificate(raw)
		if err != nil {
			return err
		}
		certs = append(certs, c)
	}

	opts := x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{usage},
	}
	if _, err := certs[0].Verify(opts); err != nil {
		return err
	}

	if len(certs[0].URIs) != 1 {
		return errors.New("exactly one SPIFFE ID is required")
	}

	uri := certs[0].URIs[0]
	if err := verifySPIFFEURI(uri, trustDomain, expectedRole); err != nil {
		return err
	}

	return nil
}

func verifySPIFFEURI(uri *url.URL, trustDomain, expectedRole string) error {
	if uri.Scheme != "spiffe" {
		return errors.New("SPIFFE ID must use spiffe:// scheme")
	}
	if uri.Host != trustDomain {
		return errors.New("SPIFFE trust domain mismatch")
	}
	path := strings.TrimPrefix(uri.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		return errors.New("invalid SPIFFE path")
	}
	role := parts[0]
	if expectedRole != "" && role != expectedRole {
		return errors.New("unexpected SPIFFE role")
	}
	return nil
}
