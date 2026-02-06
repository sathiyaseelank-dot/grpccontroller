package main

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"os"

	"controller/api"
	"controller/ca"
	controllerpb "controller/gen/controllerpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// ---- required environment variables ----
	caCertPEM := []byte(os.Getenv("INTERNAL_CA_CERT"))
	caKeyPEM := []byte(os.Getenv("INTERNAL_CA_KEY"))

	controllerCertPEM := []byte(os.Getenv("CONTROLLER_CERT"))
	controllerKeyPEM := []byte(os.Getenv("CONTROLLER_KEY"))

	if len(caCertPEM) == 0 || len(caKeyPEM) == 0 {
		log.Fatal("INTERNAL_CA_CERT or INTERNAL_CA_KEY is not set")
	}
	if len(controllerCertPEM) == 0 || len(controllerKeyPEM) == 0 {
		log.Fatal("CONTROLLER_CERT or CONTROLLER_KEY is not set")
	}

	// ---- load internal CA ----
	caInst, err := ca.LoadCA(caCertPEM, caKeyPEM)
	if err != nil {
		log.Fatalf("failed to load internal CA: %v", err)
	}

	// ---- load controller TLS certificate ----
	controllerTLSCert, err := tls.X509KeyPair(
		controllerCertPEM,
		controllerKeyPEM,
	)
	if err != nil {
		log.Fatalf("failed to load controller TLS cert: %v", err)
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
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)

	// ---- gRPC server ----
	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(api.UnarySPIFFEInterceptor("mycorp.internal")),
		grpc.StreamInterceptor(api.StreamSPIFFEInterceptor("mycorp.internal")),
	)

	// ---- enrollment service ----
	enrollServer := api.NewEnrollmentServer(
		caInst,
		caCertPEM,
		"mycorp.internal", // SPIFFE trust domain (without scheme)
	)

	controllerpb.RegisterEnrollmentServiceServer(
		grpcServer,
		enrollServer,
	)

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
