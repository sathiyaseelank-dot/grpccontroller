package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"controller/admin"
	"controller/api"
	"controller/ca"
	controllerpb "controller/gen/controllerpb"
	"controller/state"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// ---- required environment variables ----
	caCertPEM := []byte(os.Getenv("INTERNAL_CA_CERT"))
	caKeyPEM := []byte(os.Getenv("INTERNAL_CA_KEY"))
	if len(caCertPEM) == 0 || len(caKeyPEM) == 0 {
		caCertPEM, caKeyPEM = loadCAFromFiles(caCertPEM, caKeyPEM)
	}
	trustDomain := os.Getenv("TRUST_DOMAIN")
	if trustDomain == "" {
		trustDomain = "mycorp.internal"
	}
	trustDomain = normalizeTrustDomain(trustDomain)
	adminAddr := os.Getenv("ADMIN_HTTP_ADDR")
	if adminAddr == "" {
		adminAddr = ":8080"
	}
	adminAuthToken := os.Getenv("ADMIN_AUTH_TOKEN")
	internalAuthToken := os.Getenv("INTERNAL_API_TOKEN")

	if len(caCertPEM) == 0 || len(caKeyPEM) == 0 {
		log.Fatal("INTERNAL_CA_CERT or INTERNAL_CA_KEY is not set and ca/ca.crt+ca/ca.key not found")
	}
	if adminAuthToken == "" {
		log.Fatal("ADMIN_AUTH_TOKEN is not set")
	}
	if internalAuthToken == "" {
		log.Fatal("INTERNAL_API_TOKEN is not set")
	}

	// ---- load internal CA ----
	caInst, err := ca.LoadCA(caCertPEM, caKeyPEM)
	if err != nil {
		log.Fatalf("failed to load internal CA: %v", err)
	}

	// ---- load or issue controller TLS certificate ----
	controllerTLSCert, err := loadOrIssueControllerCert(caInst, trustDomain)
	if err != nil {
		log.Fatalf("failed to prepare controller TLS cert: %v", err)
	}

	// ---- build CA pool ----
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertPEM) {
		log.Fatal("failed to append internal CA cert to pool")
	}

	// ---- TLS config (mTLS enforced) ----
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{controllerTLSCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		MinVersion:   tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)

	registry := state.NewRegistry()
	tokenStore := state.NewTokenStore(10 * time.Minute)

	// ---- gRPC server ----
	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(api.UnaryAuthInterceptor(trustDomain, map[string]struct{}{
			controllerpb.EnrollmentService_EnrollConnector_FullMethodName: {},
		}, "connector", "tunneler")),
		grpc.StreamInterceptor(api.StreamSPIFFEInterceptor(trustDomain, "connector", "tunneler")),
	)

	// ---- enrollment service ----
	enrollServer := api.NewEnrollmentServer(
		caInst,
		caCertPEM,
		trustDomain, // SPIFFE trust domain (without scheme)
		tokenStore,
		registry,
	)

	controllerpb.RegisterEnrollmentServiceServer(
		grpcServer,
		enrollServer,
	)

	controllerpb.RegisterControlPlaneServer(
		grpcServer,
		api.NewControlPlaneServer(trustDomain, registry),
	)

	// ---- admin HTTP server ----
	adminMux := http.NewServeMux()
	adminServer := &admin.Server{
		Tokens:            tokenStore,
		Reg:               registry,
		AdminAuthToken:    adminAuthToken,
		InternalAuthToken: internalAuthToken,
	}
	adminServer.RegisterRoutes(adminMux)
	go func() {
		log.Printf("admin HTTP server listening on %s", adminAddr)
		if err := http.ListenAndServe(adminAddr, adminMux); err != nil {
			log.Fatalf("admin HTTP server failed: %v", err)
		}
	}()

	// ---- listen ----
	lis, err := net.Listen("tcp", ":8443")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println("controller gRPC server listening on :8443")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func loadCAFromFiles(certPEM, keyPEM []byte) ([]byte, []byte) {
	certPath := "ca/ca.crt"
	keyPath := "ca/ca.pkcs8.key"

	if len(certPEM) == 0 {
		if b, err := os.ReadFile(certPath); err == nil {
			certPEM = b
		}
	}
	if len(keyPEM) == 0 {
		if b, err := os.ReadFile(keyPath); err == nil {
			keyPEM = b
		}
	}
	return certPEM, keyPEM
}

func normalizeTrustDomain(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, ".")
	return v
}

func loadOrIssueControllerCert(caInst *ca.CA, trustDomain string) (tls.Certificate, error) {
	controllerCertPEM := []byte(os.Getenv("CONTROLLER_CERT"))
	controllerKeyPEM := []byte(os.Getenv("CONTROLLER_KEY"))
	if len(controllerCertPEM) > 0 && len(controllerKeyPEM) > 0 {
		return tls.X509KeyPair(controllerCertPEM, controllerKeyPEM)
	}

	controllerID := os.Getenv("CONTROLLER_ID")
	if controllerID == "" {
		controllerID = "default"
	}
	spiffeID := "spiffe://" + trustDomain + "/controller/" + controllerID

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM, err := ca.IssueWorkloadCert(caInst, spiffeID, &privKey.PublicKey, 12*time.Hour, []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")})
	if err != nil {
		return tls.Certificate{}, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return tls.Certificate{}, errors.New("failed to decode controller certificate")
	}

	return tls.Certificate{
		Certificate: [][]byte{block.Bytes},
		PrivateKey:  privKey,
	}, nil
}
