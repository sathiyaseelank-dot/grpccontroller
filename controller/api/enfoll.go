package api

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	controllerpb "controller/gen/controllerpb"

	"controller/ca"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnrollmentServer implements controller.v1.EnrollmentService.
type EnrollmentServer struct {
	controllerpb.UnimplementedEnrollmentServiceServer

	CA          *ca.CA
	CAPEM       []byte
	TrustDomain string
}

// NewEnrollmentServer creates a new EnrollmentServer.
func NewEnrollmentServer(caInst *ca.CA, caPEM []byte, trustDomain string) *EnrollmentServer {
	return &EnrollmentServer{
		CA:          caInst,
		CAPEM:       caPEM,
		TrustDomain: trustDomain,
	}
}

// EnrollConnector enrolls a connector and issues a short-lived certificate.
func (s *EnrollmentServer) EnrollConnector(
	ctx context.Context,
	req *controllerpb.EnrollRequest,
) (*controllerpb.EnrollResponse, error) {

	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing connector id")
	}

	pubKey, err := parsePublicKey(req.GetPublicKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	spiffeID := fmt.Sprintf(
		"spiffe://%s/connector/%s",
		s.TrustDomain,
		req.GetId(),
	)

	certPEM, err := ca.IssueWorkloadCert(
		s.CA,
		spiffeID,
		pubKey,
		1*time.Hour,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "certificate issuance failed: %v", err)
	}

	return &controllerpb.EnrollResponse{
		Certificate:   certPEM,
		CaCertificate: s.CAPEM,
	}, nil
}

// EnrollTunneler enrolls a tunneler and issues a short-lived certificate.
func (s *EnrollmentServer) EnrollTunneler(
	ctx context.Context,
	req *controllerpb.EnrollRequest,
) (*controllerpb.EnrollResponse, error) {

	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing tunneler id")
	}

	pubKey, err := parsePublicKey(req.GetPublicKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	spiffeID := fmt.Sprintf(
		"spiffe://%s/tunneler/%s",
		s.TrustDomain,
		req.GetId(),
	)

	certPEM, err := ca.IssueWorkloadCert(
		s.CA,
		spiffeID,
		pubKey,
		30*time.Minute,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "certificate issuance failed: %v", err)
	}

	return &controllerpb.EnrollResponse{
		Certificate:   certPEM,
		CaCertificate: s.CAPEM,
	}, nil
}

// Renew re-issues a certificate for an existing workload.
// This example assumes the same semantics as EnrollTunneler.
// In a real system, you would authenticate the caller first.
func (s *EnrollmentServer) Renew(
	ctx context.Context,
	req *controllerpb.EnrollRequest,
) (*controllerpb.EnrollResponse, error) {

	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing id")
	}

	pubKey, err := parsePublicKey(req.GetPublicKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	// NOTE: For simplicity, renewal is treated as a tunneler renewal here.
	// You can split this later based on authenticated role.
	spiffeID := fmt.Sprintf(
		"spiffe://%s/tunneler/%s",
		s.TrustDomain,
		req.GetId(),
	)

	certPEM, err := ca.IssueWorkloadCert(
		s.CA,
		spiffeID,
		pubKey,
		30*time.Minute,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "certificate renewal failed: %v", err)
	}

	return &controllerpb.EnrollResponse{
		Certificate:   certPEM,
		CaCertificate: s.CAPEM,
	}, nil
}

// parsePublicKey parses a PEM-encoded public key.
func parsePublicKey(pemBytes []byte) (interface{}, error) {
	if len(pemBytes) == 0 {
		return nil, fmt.Errorf("public key is empty")
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return pub, nil
}
