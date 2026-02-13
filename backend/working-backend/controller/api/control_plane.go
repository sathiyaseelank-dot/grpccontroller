package api

import (
	"encoding/json"
	"io"
	"log"
	"sync"

	controllerpb "controller/gen/controllerpb"
	"controller/state"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ControlPlaneServer implements the controller.v1.ControlPlane service.
type ControlPlaneServer struct {
	controllerpb.UnimplementedControlPlaneServer
	registry       *state.Registry
	tunnelers      *state.TunnelerRegistry
	tunnelerStatus *state.TunnelerStatusRegistry
	mu             sync.Mutex
	clients        map[string]*connectorClient
}

// NewControlPlaneServer creates a new control plane server.
func NewControlPlaneServer(trustDomain string, registry *state.Registry, tunnelers *state.TunnelerRegistry, tunnelerStatus *state.TunnelerStatusRegistry) *ControlPlaneServer {
	_ = trustDomain
	return &ControlPlaneServer{
		registry:       registry,
		tunnelers:      tunnelers,
		tunnelerStatus: tunnelerStatus,
		clients:        make(map[string]*connectorClient),
	}
}

// Connect handles a persistent control-plane stream from connectors.
func (s *ControlPlaneServer) Connect(stream controllerpb.ControlPlane_ConnectServer) error {
	role, ok := RoleFromContext(stream.Context())
	if !ok || role != "connector" {
		return status.Error(codes.PermissionDenied, "connector role required")
	}

	spiffeID, _ := SPIFFEIDFromContext(stream.Context())
	log.Printf("control-plane stream connected: %s", spiffeID)
	client := &connectorClient{stream: stream}
	s.addClient(spiffeID, client)
	defer s.removeClient(spiffeID)
	s.sendAllowlist(client)

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
		if msg.GetType() == "tunneler_heartbeat" && s.tunnelerStatus != nil {
			var payload struct {
				TunnelerID  string `json:"tunneler_id"`
				SPIFFEID    string `json:"spiffe_id"`
				Status      string `json:"status"`
				ConnectorID string `json:"connector_id"`
			}
			if err := json.Unmarshal(msg.GetPayload(), &payload); err == nil {
				s.tunnelerStatus.Record(payload.TunnelerID, payload.SPIFFEID, payload.ConnectorID)
			}
		}
	}
}

// NotifyTunnelerAllowed broadcasts a newly enrolled tunneler to all connectors.
func (s *ControlPlaneServer) NotifyTunnelerAllowed(tunnelerID, spiffeID string) {
	if s.tunnelers != nil {
		s.tunnelers.Add(tunnelerID, spiffeID)
	}
	info := state.TunnelerInfo{ID: tunnelerID, SPIFFEID: spiffeID}
	payload, err := json.Marshal(info)
	if err != nil {
		return
	}
	s.broadcast(&controllerpb.ControlMessage{
		Type:    "tunneler_allow",
		Payload: payload,
	})
}

type connectorClient struct {
	stream controllerpb.ControlPlane_ConnectServer
	sendMu sync.Mutex
}

func (s *ControlPlaneServer) addClient(id string, c *connectorClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[id] = c
}

func (s *ControlPlaneServer) removeClient(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, id)
}

func (s *ControlPlaneServer) broadcast(msg *controllerpb.ControlMessage) {
	s.mu.Lock()
	clients := make([]*connectorClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	for _, c := range clients {
		c.sendMu.Lock()
		_ = c.stream.Send(msg)
		c.sendMu.Unlock()
	}
}

func (s *ControlPlaneServer) sendAllowlist(c *connectorClient) {
	if s.tunnelers == nil {
		return
	}
	list := s.tunnelers.List()
	payload, err := json.Marshal(list)
	if err != nil {
		return
	}
	c.sendMu.Lock()
	_ = c.stream.Send(&controllerpb.ControlMessage{
		Type:    "tunneler_allowlist",
		Payload: payload,
	})
	c.sendMu.Unlock()
}
