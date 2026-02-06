package ca

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// CA represents the internal Certificate Authority used by the controller.
// It holds the parsed CA certificate and a crypto.Signer for the CA private key.
type CA struct {
	Cert *x509.Certificate
	Key  crypto.Signer
}

// LoadCA loads and parses the internal CA certificate and private key.
// certPEM and keyPEM must be PEM-encoded data.
// The private key must implement crypto.Signer (RSA, ECDSA, TPM-backed, etc.).
func LoadCA(certPEM, keyPEM []byte) (*CA, error) {
	if len(certPEM) == 0 {
		return nil, errors.New("CA certificate PEM is empty")
	}
	if len(keyPEM) == 0 {
		return nil, errors.New("CA private key PEM is empty")
	}

	// Decode and parse CA certificate
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, errors.New("failed to decode CA certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}

	// Decode and parse CA private key (PKCS#8)
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, errors.New("failed to decode CA private key PEM")
	}

	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("CA private key does not implement crypto.Signer")
	}

	return &CA{
		Cert: cert,
		Key:  signer,
	}, nil
}
