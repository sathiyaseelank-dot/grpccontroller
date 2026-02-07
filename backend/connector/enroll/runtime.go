package enroll

import (
	"fmt"
	"net"
	"os"
	"strings"

	"connector/internal/buildinfo"
)

const (
	privateIPEnv = "CONNECTOR_PRIVATE_IP"
	versionEnv   = "CONNECTOR_VERSION"
)

func ResolveVersion() string {
	if v := strings.TrimSpace(os.Getenv(versionEnv)); v != "" {
		return v
	}
	if v := strings.TrimSpace(buildinfo.Version); v != "" {
		return v
	}
	return "unknown"
}

func ResolvePrivateIP(controllerAddr string) (string, error) {
	if ip := strings.TrimSpace(os.Getenv(privateIPEnv)); ip != "" {
		return ip, nil
	}
	ip, err := discoverPrivateIP(controllerAddr)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func discoverPrivateIP(controllerAddr string) (string, error) {
	host := controllerAddr
	if strings.Contains(controllerAddr, "://") {
		return "", fmt.Errorf("CONTROLLER_ADDR must be host:port")
	}
	if h, _, err := net.SplitHostPort(controllerAddr); err == nil && h != "" {
		host = h
	}
	conn, err := net.Dial("udp", net.JoinHostPort(host, "53"))
	if err != nil {
		return "", fmt.Errorf("failed to determine private IP: %w", err)
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr.IP == nil {
		return "", fmt.Errorf("failed to determine private IP")
	}
	return localAddr.IP.String(), nil
}
