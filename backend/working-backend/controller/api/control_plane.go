package api

import (
	"io"
	"log"

	controllerpb "controller/gen/controllerpb"
	"controller/state"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ControlPlaneServer implements the controller.v1.ControlPlane service.
type ControlPlaneServer struct {
	controllerpb.UnimplementedControlPlaneServer
	registry *state.Registry
}

// NewControlPlaneServer creates a new control plane server.
func NewControlPlaneServer(trustDomain string, registry *state.Registry) *ControlPlaneServer {
	_ = trustDomain
	return &ControlPlaneServer{registry: registry}
}

// Connect handles a persistent control-plane stream from connectors.
func (s *ControlPlaneServer) Connect(stream controllerpb.ControlPlane_ConnectServer) error {
	role, ok := RoleFromContext(stream.Context())
	if !ok || role != "connector" {
		return status.Error(codes.PermissionDenied, "connector role required")
	}

	spiffeID, _ := SPIFFEIDFromContext(stream.Context())
	log.Printf("control-plane stream connected: %s", spiffeID)

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
		if msg.GetType() == "heartbeat" {
			if s.registry != nil {
				s.registry.RecordHeartbeat(msg.GetConnectorId(), msg.GetPrivateIp())
			}
			log.Printf("heartbeat: connector_id=%s private_ip=%s status=%s", msg.GetConnectorId(), msg.GetPrivateIp(), msg.GetStatus())
		}
	}
}
