package enroll

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"connector/internal/tlsutil"
	controllerpb "controller/gen/controllerpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Config controls enrollment behavior.
type Config struct {
	ControllerAddr string
	ConnectorID    string
	TrustDomain    string
	RootCAPEM      []byte
	BootstrapCert  []byte
	BootstrapKey   []byte
}

// Run performs one-time connector enrollment with the controller.
func Run() error {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cert, _, caPEM, spiffeID, err := Enroll(ctx, cfg)
	if err != nil {
		return err
	}

	_ = cert
	_ = caPEM
	fmt.Printf("Enrolled connector with SPIFFE ID: %s\n", spiffeID)
	return nil
}

// ConfigFromEnv builds Config from environment variables.
func ConfigFromEnv() (Config, error) {
	controllerAddr := os.Getenv("CONTROLLER_ADDR")
	connectorID := os.Getenv("CONNECTOR_ID")
	trustDomain := os.Getenv("TRUST_DOMAIN")
	if trustDomain == "" {
		trustDomain = "mycorp.internal"
	}

	rootCAPEM := []byte(os.Getenv("INTERNAL_CA_CERT"))
	bootstrapCert := []byte(os.Getenv("BOOTSTRAP_CERT"))
	bootstrapKey := []byte(os.Getenv("BOOTSTRAP_KEY"))

	if controllerAddr == "" {
		return Config{}, fmt.Errorf("CONTROLLER_ADDR is not set")
	}
	if connectorID == "" {
		return Config{}, fmt.Errorf("CONNECTOR_ID is not set")
	}
	if len(rootCAPEM) == 0 {
		return Config{}, fmt.Errorf("INTERNAL_CA_CERT is not set")
	}
	if len(bootstrapCert) == 0 || len(bootstrapKey) == 0 {
		return Config{}, fmt.Errorf("BOOTSTRAP_CERT or BOOTSTRAP_KEY is not set")
	}

	return Config{
		ControllerAddr: controllerAddr,
		ConnectorID:    connectorID,
		TrustDomain:    trustDomain,
		RootCAPEM:      rootCAPEM,
		BootstrapCert:  bootstrapCert,
		BootstrapKey:   bootstrapKey,
	}, nil
}

// Enroll performs enrollment and returns the issued workload certificate.
func Enroll(ctx context.Context, cfg Config) (tls.Certificate, []byte, []byte, string, error) {
	// ---- generate key pair (in-memory only) ----
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("failed to generate key pair: %w", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	bootstrapCert, err := tls.X509KeyPair(cfg.BootstrapCert, cfg.BootstrapKey)
	if err != nil {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("invalid bootstrap certificate: %w", err)
	}

	rootPool, err := tlsutil.RootPoolFromPEM(cfg.RootCAPEM)
	if err != nil {
		return tls.Certificate{}, nil, nil, "", err
	}

	// ---- TLS config for enrollment ----
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{bootstrapCert},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return tlsutil.VerifyPeerSPIFFE(rawCerts, rootPool, cfg.TrustDomain, "controller", x509.ExtKeyUsageServerAuth)
		},
	}

	// ---- connect to controller ----
	conn, err := grpc.DialContext(
		ctx,
		cfg.ControllerAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer conn.Close()

	client := controllerpb.NewEnrollmentServiceClient(conn)

	resp, err := client.EnrollConnector(ctx, &controllerpb.EnrollRequest{
		Id:        cfg.ConnectorID,
		PublicKey: pubPEM,
	})
	if err != nil {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("enrollment RPC failed: %w", err)
	}

	if len(resp.Certificate) == 0 {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("controller returned empty certificate")
	}
	if len(resp.CaCertificate) == 0 {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("controller returned empty CA certificate")
	}

	// ---- basic validation of returned cert ----
	block, _ := pem.Decode(resp.Certificate)
	if block == nil || block.Type != "CERTIFICATE" {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("invalid certificate PEM from controller")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("failed to parse issued certificate: %w", err)
	}

	if len(cert.URIs) != 1 {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("issued certificate must contain exactly one SPIFFE ID")
	}

	// ---- ensure CA pinning ----
	if !equalCAPEM(cfg.RootCAPEM, resp.CaCertificate) {
		return tls.Certificate{}, nil, nil, "", fmt.Errorf("controller CA does not match pinned CA")
	}

	workloadCert := tls.Certificate{
		Certificate: [][]byte{block.Bytes},
		PrivateKey:  privKey,
	}

	return workloadCert, resp.Certificate, resp.CaCertificate, cert.URIs[0].String(), nil
}

func equalCAPEM(a, b []byte) bool {
	ab, _ := pem.Decode(a)
	bb, _ := pem.Decode(b)
	if ab == nil || bb == nil {
		return false
	}
	return string(ab.Bytes) == string(bb.Bytes)
}
