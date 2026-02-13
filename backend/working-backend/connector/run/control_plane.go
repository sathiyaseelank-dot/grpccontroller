package run

import (
	"encoding/json"
	"io"
	"log"
	"strings"

	"connector/internal/spiffe"
	controllerpb "controller/gen/controllerpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type controlPlaneServer struct {
	controllerpb.UnimplementedControlPlaneServer
	connectorID string
	sendCh      chan<- *controllerpb.ControlMessage
}

func (s *controlPlaneServer) Connect(stream controllerpb.ControlPlane_ConnectServer) error {
	role, ok := spiffe.RoleFromContext(stream.Context())
	if !ok || role != "tunneler" {
		return status.Error(codes.PermissionDenied, "tunneler role required")
	}

	spiffeID, _ := spiffe.SPIFFEIDFromContext(stream.Context())
	log.Printf("tunneler connected: %s", spiffeID)
	tunnelerID := parseTunnelerID(spiffeID)

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
		if msg.GetType() == "tunneler_heartbeat" && s.sendCh != nil {
			payload := struct {
				TunnelerID  string `json:"tunneler_id"`
				SPIFFEID    string `json:"spiffe_id"`
				Status      string `json:"status"`
				ConnectorID string `json:"connector_id"`
			}{
				TunnelerID:  tunnelerID,
				SPIFFEID:    spiffeID,
				Status:      msg.GetStatus(),
				ConnectorID: s.connectorID,
			}
			if data, err := json.Marshal(payload); err == nil {
				s.sendCh <- &controllerpb.ControlMessage{
					Type:    "tunneler_heartbeat",
					Payload: data,
				}
			}
		}
	}
}

func parseTunnelerID(spiffeID string) string {
	if spiffeID == "" {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(spiffeID, "spiffe://"), "/")
	if len(parts) < 3 {
		return ""
	}
	if parts[1] != "tunneler" {
		return ""
	}
	return parts[2]
}
