package core

import (
	"fmt"
	"net"
)

// IsPortAvailable checks if a TCP port is currently free to bind on localhost.
func IsPortAvailable(port int) bool {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

// FindNextFreePort finds the lowest available port >= startPort that is neither
// claimed in existing instance configurations nor bound by another process.
func FindNextFreePort(startPort int, existingInstances []*DatabaseInstance) int {
	usedPorts := make(map[int]bool)
	for _, inst := range existingInstances {
		if inst.Port > 0 {
			usedPorts[inst.Port] = true
		}
	}

	port := startPort
	for {
		if !usedPorts[port] && IsPortAvailable(port) {
			return port
		}
		port++
		if port > 65535 {
			return startPort
		}
	}
}
