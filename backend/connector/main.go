package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"connector/enroll"
	"connector/run"
)

const configPath = "/etc/grpcconnector/connector.conf"

func main() {
	if err := loadConfig(configPath); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if len(os.Args) < 2 {
		log.Fatal("missing command: enroll | run")
	}

	switch os.Args[1] {
	case "enroll":
		if err := enroll.Run(); err != nil {
			log.Fatalf("enrollment failed: %v", err)
		}
		log.Println("enrollment completed successfully")

	case "run":
		if err := run.Run(); err != nil {
			log.Fatalf("connector run failed: %v", err)
		}

	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid config line: %q", line)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			return fmt.Errorf("invalid config line: %q", line)
		}
		if err := os.Setenv(key, val); err != nil {
			return fmt.Errorf("setenv %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}
