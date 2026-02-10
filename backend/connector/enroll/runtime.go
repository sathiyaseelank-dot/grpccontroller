package enroll

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connector/internal/buildinfo"
)

const (
	privateIPEnv = "CONNECTOR_PRIVATE_IP"
	versionEnv   = "CONNECTOR_VERSION"

	identityDir     = "/etc/grpcconnector"
	identityCertPem = "connector.crt"
	identityKeyPem  = "connector.key"
	identityCAPem   = "controller.ca"
)

func ResolveVersion() string {
	if v := strings.TrimSpace(os.Getenv(versionEnv)); v != "" {
		return v
	}
	if v := strings.TrimSpace(buildinfo.Version); v != "" {
		return v
	}
	return "unknown"
}

func ResolvePrivateIP(controllerAddr string) (string, error) {
	if ip := strings.TrimSpace(os.Getenv(privateIPEnv)); ip != "" {
		return ip, nil
	}
	ip, err := discoverPrivateIP(controllerAddr)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func discoverPrivateIP(controllerAddr string) (string, error) {
	host, err := controllerHost(controllerAddr)
	if err != nil {
		return "", err
	}
	conn, err := net.Dial("udp", net.JoinHostPort(host, "53"))
	if err != nil {
		return "", fmt.Errorf("failed to determine private IP: %w", err)
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr.IP == nil {
		return "", fmt.Errorf("failed to determine private IP")
	}
	return localAddr.IP.String(), nil
}

func controllerHost(controllerAddr string) (string, error) {
	if strings.Contains(controllerAddr, "://") {
		return "", fmt.Errorf("CONTROLLER_ADDR must be host:port")
	}
	if host, _, err := net.SplitHostPort(controllerAddr); err == nil && host != "" {
		return host, nil
	}
	return "", fmt.Errorf("CONTROLLER_ADDR must be host:port")
}

func normalizeTrustDomain(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, ".")
	return v
}

func loadExplicitCA() ([]byte, error) {
	caPath := strings.TrimSpace(os.Getenv("CONTROLLER_CA"))
	if caPath == "" {
		return nil, fmt.Errorf(
			"CONTROLLER_CA is not set (explicit controller trust is required)",
		)
	}

	pemBytes, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read controller CA at %s: %w", caPath, err)
	}

	return pemBytes, nil
}

func IdentityPaths() (certPath, keyPath, caPath string) {
	certPath = filepath.Join(identityDir, identityCertPem)
	keyPath = filepath.Join(identityDir, identityKeyPem)
	caPath = filepath.Join(identityDir, identityCAPem)
	return
}

func LoadIdentity() (tls.Certificate, []byte, time.Time, error) {
	certPath, keyPath, caPath := IdentityPaths()

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}

	workloadCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return tls.Certificate{}, nil, time.Time{}, fmt.Errorf("invalid stored certificate PEM")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}

	return workloadCert, caPEM, leaf.NotAfter, nil
}

func PersistIdentity(cert tls.Certificate, caPEM []byte) error {
	if err := os.MkdirAll(identityDir, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %w", identityDir, err)
	}
	if err := os.Chmod(identityDir, 0700); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", identityDir, err)
	}

	certPath, keyPath, caPath := IdentityPaths()

	if len(cert.Certificate) == 0 {
		return fmt.Errorf("missing certificate data")
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	})

	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}
	if err := os.WriteFile(caPath, caPEM, 0600); err != nil {
		return fmt.Errorf("failed to write CA: %w", err)
	}

	return nil
}
