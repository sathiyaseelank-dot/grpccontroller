package run

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"connector/enroll"
	"connector/internal/spiffe"
	"connector/internal/tlsutil"
	controllerpb "controller/gen/controllerpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Run starts the long-running connector service.
func Run() error {
	cfg, err := configFromEnv()
	if err != nil {
		return err
	}

	enrollCfg, err := enroll.ConfigFromEnvRun()
	if err != nil {
		return err
	}
	enrollCfg.Token = os.Getenv("ENROLLMENT_TOKEN")
	if enrollCfg.Token == "" {
		cred, err := enroll.ReadCredential("ENROLLMENT_TOKEN")
		if err != nil {
			return err
		}
		enrollCfg.Token = cred
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if systemdWatchdogEnabled() {
		go systemdWatchdogLoop(ctx)
	}

	if enrollCfg.Token == "" {
		return fmt.Errorf("ENROLLMENT_TOKEN is required for enrollment")
	}
	cert, certPEM, caPEM, spiffeID, err := enroll.Enroll(ctx, enrollCfg)
	if err != nil {
		return err
	}

	certInfo, err := parseLeafCert(certPEM)
	if err != nil {
		return err
	}
	workloadCert := cert
	notAfter := certInfo.NotAfter

	log.Printf("connector enrolled as %s", spiffeID)

	store := tlsutil.NewCertStore(workloadCert, nil, notAfter)
	rootPool, err := tlsutil.RootPoolFromPEM(caPEM)
	if err != nil {
		return err
	}

	reloadCh := make(chan struct{}, 1)
	go controlPlaneLoop(ctx, cfg.controllerAddr, cfg.trustDomain, cfg.connectorID, cfg.privateIP, store, rootPool, reloadCh)
	go renewalLoop(ctx, cfg.controllerAddr, cfg.connectorID, cfg.trustDomain, store, rootPool, caPEM, reloadCh)

	if cfg.listenAddr != "" {
		go serverLoop(ctx, cfg.listenAddr, cfg.trustDomain, store, rootPool)
	}

	<-ctx.Done()
	return ctx.Err()
}

func systemdWatchdogEnabled() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--systemd-watchdog" {
			return true
		}
	}
	return false
}

func systemdWatchdogLoop(ctx context.Context) {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" {
		return
	}
	interval := watchdogInterval()
	if interval <= 0 {
		return
	}

	if err := systemdNotify(socket, "READY=1"); err != nil {
		log.Printf("systemd notify failed: %v", err)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = systemdNotify(socket, "WATCHDOG=1")
		}
	}
}

func watchdogInterval() time.Duration {
	usecStr := strings.TrimSpace(os.Getenv("WATCHDOG_USEC"))
	if usecStr == "" {
		return 0
	}
	usec, err := strconv.ParseInt(usecStr, 10, 64)
	if err != nil || usec <= 0 {
		return 0
	}
	d := time.Duration(usec) * time.Microsecond
	return d / 2
}

func systemdNotify(socket, msg string) error {
	addr := socket
	if strings.HasPrefix(addr, "@") {
		addr = "\x00" + addr[1:]
	}
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: addr, Net: "unixgram"})
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(msg))
	return err
}

type runtimeConfig struct {
	controllerAddr string
	connectorID    string
	trustDomain    string
	listenAddr     string
	privateIP      string
}

func configFromEnv() (runtimeConfig, error) {
	controllerAddr := os.Getenv("CONTROLLER_ADDR")
	connectorID := os.Getenv("CONNECTOR_ID")
	trustDomain := os.Getenv("TRUST_DOMAIN")
	listenAddr := os.Getenv("CONNECTOR_LISTEN_ADDR")

	if trustDomain == "" {
		trustDomain = "mycorp.internal"
	}
	if controllerAddr == "" {
		return runtimeConfig{}, fmt.Errorf("CONTROLLER_ADDR is not set")
	}
	if connectorID == "" {
		return runtimeConfig{}, fmt.Errorf("CONNECTOR_ID is not set")
	}

	privateIP, err := enroll.ResolvePrivateIP(controllerAddr)
	if err != nil {
		return runtimeConfig{}, err
	}

	return runtimeConfig{
		controllerAddr: controllerAddr,
		connectorID:    connectorID,
		trustDomain:    trustDomain,
		listenAddr:     listenAddr,
		privateIP:      privateIP,
	}, nil
}

func runConnectorServer(addr, trustDomain string, store *tlsutil.CertStore, roots *x509.CertPool) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		MinVersion:     tls.VersionTLS13,
		ClientAuth:     tls.RequireAndVerifyClientCert,
		ClientCAs:      roots,
		GetCertificate: store.GetCertificate,
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.UnaryInterceptor(spiffe.UnaryInterceptor(trustDomain, "tunneler")),
		grpc.StreamInterceptor(spiffe.StreamInterceptor(trustDomain, "tunneler")),
	)

	controllerpb.RegisterControlPlaneServer(grpcServer, &controlPlaneServer{})

	log.Printf("connector server listening on %s", addr)
	return grpcServer.Serve(lis)
}

func serverLoop(ctx context.Context, addr, trustDomain string, store *tlsutil.CertStore, roots *x509.CertPool) {
	backoff := 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := runConnectorServer(addr, trustDomain, store, roots); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("connector server stopped: %v", err)
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func controlPlaneLoop(ctx context.Context, controllerAddr, trustDomain, connectorID, privateIP string, store *tlsutil.CertStore, roots *x509.CertPool, reloadCh <-chan struct{}) {
	backoff := 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sessionCtx, cancel := context.WithCancel(ctx)
		errCh := make(chan error, 1)
		go func() {
			errCh <- connectControlPlane(sessionCtx, controllerAddr, trustDomain, connectorID, privateIP, store, roots)
		}()

		select {
		case <-ctx.Done():
			cancel()
			<-errCh
			return
		case <-reloadCh:
			cancel()
			<-errCh
		case err := <-errCh:
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("control-plane connection ended: %v", err)
			}
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func connectControlPlane(ctx context.Context, controllerAddr, trustDomain, connectorID, privateIP string, store *tlsutil.CertStore, roots *x509.CertPool) error {
	tlsConfig := &tls.Config{
		MinVersion:           tls.VersionTLS13,
		GetClientCertificate: store.GetClientCertificate,
		RootCAs:              roots,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return tlsutil.VerifyPeerSPIFFE(rawCerts, verifiedChains, trustDomain, "controller")
		},
	}

	conn, err := grpc.DialContext(
		ctx,
		controllerAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := controllerpb.NewControlPlaneClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&controllerpb.ControlMessage{Type: "connector_hello"}); err != nil {
		return err
	}

	recvErr := make(chan error, 1)
	go func() {
		for {
			_, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
		}
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-recvErr:
			return err
		case <-ticker.C:
			if err := stream.Send(&controllerpb.ControlMessage{
				Type:        "heartbeat",
				ConnectorId: connectorID,
				PrivateIp:   privateIP,
				Status:      "ONLINE",
			}); err != nil {
				return err
			}
		}
	}
}

func renewalLoop(ctx context.Context, controllerAddr, connectorID, trustDomain string, store *tlsutil.CertStore, roots *x509.CertPool, caPEM []byte, reloadCh chan<- struct{}) {
	for {
		next := nextRenewal(store.NotAfter())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		cert, certPEM, notAfter, err := renewOnce(ctx, controllerAddr, connectorID, trustDomain, store, roots, caPEM)
		if err != nil {
			log.Printf("certificate renewal failed: %v", err)
			continue
		}

		store.Update(cert, certPEM, notAfter)
		select {
		case reloadCh <- struct{}{}:
		default:
		}
	}
}

func renewOnce(ctx context.Context, controllerAddr, connectorID, trustDomain string, store *tlsutil.CertStore, roots *x509.CertPool, caPEM []byte) (tls.Certificate, []byte, time.Time, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	tlsConfig := &tls.Config{
		MinVersion:           tls.VersionTLS13,
		GetClientCertificate: store.GetClientCertificate,
		RootCAs:              roots,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return tlsutil.VerifyPeerSPIFFE(rawCerts, verifiedChains, trustDomain, "controller")
		},
	}

	conn, err := grpc.DialContext(
		ctx,
		controllerAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}
	defer conn.Close()

	client := controllerpb.NewEnrollmentServiceClient(conn)
	resp, err := client.Renew(ctx, &controllerpb.EnrollRequest{Id: connectorID, PublicKey: pubPEM})
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}
	if len(resp.CaCertificate) == 0 {
		return tls.Certificate{}, nil, time.Time{}, errors.New("empty CA certificate in renewal response")
	}
	if !tlsutil.EqualCAPEM(caPEM, resp.CaCertificate) {
		return tls.Certificate{}, nil, time.Time{}, errors.New("internal CA mismatch during renewal")
	}

	block, _ := pem.Decode(resp.Certificate)
	if block == nil || block.Type != "CERTIFICATE" {
		return tls.Certificate{}, nil, time.Time{}, errors.New("invalid certificate PEM")
	}

	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return tls.Certificate{}, nil, time.Time{}, err
	}

	workloadCert := tls.Certificate{Certificate: [][]byte{block.Bytes}, PrivateKey: privKey}
	return workloadCert, resp.Certificate, leaf.NotAfter, nil
}

func nextRenewal(notAfter time.Time) time.Time {
	ttl := time.Until(notAfter)
	if ttl <= 0 {
		return time.Now().Add(10 * time.Second)
	}
	advance := ttl / 3
	if advance < 5*time.Minute {
		advance = 5 * time.Minute
	}
	next := notAfter.Add(-advance)
	if next.Before(time.Now().Add(30 * time.Second)) {
		return time.Now().Add(30 * time.Second)
	}
	return next
}

func parseLeafCert(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("invalid certificate PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}
