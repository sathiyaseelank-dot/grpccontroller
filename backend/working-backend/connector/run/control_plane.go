package run

import (
	"io"
	"log"

	"connector/internal/spiffe"
	controllerpb "controller/gen/controllerpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type controlPlaneServer struct {
	controllerpb.UnimplementedControlPlaneServer
}

func (s *controlPlaneServer) Connect(stream controllerpb.ControlPlane_ConnectServer) error {
	role, ok := spiffe.RoleFromContext(stream.Context())
	if !ok || role != "tunneler" {
		return status.Error(codes.PermissionDenied, "tunneler role required")
	}

	spiffeID, _ := spiffe.SPIFFEIDFromContext(stream.Context())
	log.Printf("tunneler connected: %s", spiffeID)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if msg.GetType() == "ping" {
			if err := stream.Send(&controllerpb.ControlMessage{Type: "pong"}); err != nil {
				return err
			}
		}
	}
}
