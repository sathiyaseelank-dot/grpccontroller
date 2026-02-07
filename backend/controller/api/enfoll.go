package api

import (
	"context"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	controllerpb "controller/gen/controllerpb"

	"controller/ca"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnrollmentServer implements controller.v1.EnrollmentService.
type EnrollmentServer struct {
	controllerpb.UnimplementedEnrollmentServiceServer

	CA                   *ca.CA
	CAPEM                []byte
	TrustDomain          string
	ConnectorEnrollToken string
}

// NewEnrollmentServer creates a new EnrollmentServer.
func NewEnrollmentServer(caInst *ca.CA, caPEM []byte, trustDomain, connectorToken string) *EnrollmentServer {
	return &EnrollmentServer{
		CA:                   caInst,
		CAPEM:                caPEM,
		TrustDomain:          trustDomain,
		ConnectorEnrollToken: connectorToken,
	}
}

// EnrollConnector enrolls a connector and issues a short-lived certificate.
func (s *EnrollmentServer) EnrollConnector(
	ctx context.Context,
	req *controllerpb.EnrollRequest,
) (*controllerpb.EnrollResponse, error) {

	if !validID(req.GetId()) {
		return nil, status.Error(codes.InvalidArgument, "missing connector id")
	}
	if req.GetPrivateIp() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing private ip")
	}
	if req.GetVersion() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing version")
	}

	pubKey, err := parsePublicKey(req.GetPublicKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	if err := s.authorizeConnectorToken(req.GetToken()); err != nil {
		return nil, err
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

	// Registration side-effect: log enrollment details.
	logEnrollment("connector", req.GetId(), req.GetPrivateIp(), req.GetVersion())

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

	if !validID(req.GetId()) {
		return nil, status.Error(codes.InvalidArgument, "missing tunneler id")
	}

	pubKey, err := parsePublicKey(req.GetPublicKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	if err := s.authorize(ctx, "tunneler", req.GetId()); err != nil {
		return nil, err
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

// Renew re-issues a certificate for an existing workload based on its SPIFFE identity.
func (s *EnrollmentServer) Renew(
	ctx context.Context,
	req *controllerpb.EnrollRequest,
) (*controllerpb.EnrollResponse, error) {

	if !validID(req.GetId()) {
		return nil, status.Error(codes.InvalidArgument, "missing id")
	}

	pubKey, err := parsePublicKey(req.GetPublicKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	role, id, err := s.identityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if id != req.GetId() {
		return nil, status.Error(codes.PermissionDenied, "id mismatch for renewal")
	}

	spiffeID := fmt.Sprintf("spiffe://%s/%s/%s", s.TrustDomain, role, req.GetId())

	ttl := 30 * time.Minute
	if role == "connector" {
		ttl = 1 * time.Hour
	}

	certPEM, err := ca.IssueWorkloadCert(s.CA, spiffeID, pubKey, ttl)
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

func (s *EnrollmentServer) authorize(ctx context.Context, expectedRole, expectedID string) error {
	role, id, err := s.identityFromContext(ctx)
	if err != nil {
		return err
	}
	if role != expectedRole {
		return status.Error(codes.PermissionDenied, "role not permitted for enrollment")
	}
	if id != expectedID {
		return status.Error(codes.PermissionDenied, "id mismatch for enrollment")
	}
	return nil
}

func (s *EnrollmentServer) authorizeConnectorToken(provided string) error {
	if s.ConnectorEnrollToken == "" {
		return status.Error(codes.FailedPrecondition, "connector enrollment token not configured")
	}
	if provided == "" {
		return status.Error(codes.Unauthenticated, "missing enrollment token")
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.ConnectorEnrollToken)) != 1 {
		return status.Error(codes.PermissionDenied, "invalid enrollment token")
	}
	return nil
}

func (s *EnrollmentServer) identityFromContext(ctx context.Context) (string, string, error) {
	spiffeID, ok := SPIFFEIDFromContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "missing SPIFFE identity")
	}

	role, ok := RoleFromContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "missing SPIFFE role")
	}

	id := strings.TrimPrefix(spiffeID, fmt.Sprintf("spiffe://%s/%s/", s.TrustDomain, role))
	if id == "" || strings.Contains(id, "/") {
		return "", "", status.Error(codes.Unauthenticated, "invalid SPIFFE id")
	}

	return role, id, nil
}

func logEnrollment(role, id, privateIP, version string) {
	// Keep as a structured line to aid operator log parsing.
	fmt.Printf("enrollment: role=%s id=%s private_ip=%s version=%s\n", role, id, privateIP, version)
}

func validID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}
