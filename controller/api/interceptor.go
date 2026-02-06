package api

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// contextKey is a private type to avoid collisions in context.
type contextKey string

const (
	spiffeIDContextKey contextKey = "spiffe-id"
	roleContextKey     contextKey = "spiffe-role"
)

// UnarySPIFFEInterceptor enforces SPIFFE identity on unary RPCs.
func UnarySPIFFEInterceptor(trustDomain string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		spiffeID, role, err := extractAndVerifySPIFFE(ctx, trustDomain)
		if err != nil {
			return nil, err
		}

		ctx = context.WithValue(ctx, spiffeIDContextKey, spiffeID)
		ctx = context.WithValue(ctx, roleContextKey, role)

		return handler(ctx, req)
	}
}

// StreamSPIFFEInterceptor enforces SPIFFE identity on streaming RPCs.
func StreamSPIFFEInterceptor(trustDomain string) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {

		spiffeID, role, err := extractAndVerifySPIFFE(ss.Context(), trustDomain)
		if err != nil {
			return err
		}

		wrapped := &wrappedStream{
			ServerStream: ss,
			ctx: context.WithValue(
				context.WithValue(ss.Context(), spiffeIDContextKey, spiffeID),
				roleContextKey,
				role,
			),
		}

		return handler(srv, wrapped)
	}
}

// wrappedStream allows us to override Context().
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// extractAndVerifySPIFFE pulls the peer certificate from context and validates
// the SPIFFE ID and role.
func extractAndVerifySPIFFE(ctx context.Context, trustDomain string) (string, string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", "", errors.New("missing peer information")
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return "", "", errors.New("connection is not using TLS")
	}

	if len(tlsInfo.State.PeerCertificates) == 0 {
		return "", "", errors.New("no peer certificates presented")
	}

	cert := tlsInfo.State.PeerCertificates[0]

	if len(cert.URIs) != 1 {
		return "", "", errors.New("exactly one SPIFFE ID is required")
	}

	uri := cert.URIs[0]

	if uri.Scheme != "spiffe" {
		return "", "", errors.New("SPIFFE ID must use spiffe:// scheme")
	}

	if uri.Host != trustDomain {
		return "", "", errors.New("SPIFFE trust domain mismatch")
	}

	path := strings.TrimPrefix(uri.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", errors.New("invalid SPIFFE path format")
	}

	role := parts[0]
	if role != "connector" && role != "tunneler" {
		return "", "", errors.New("invalid SPIFFE role")
	}

	return uri.String(), role, nil
}
