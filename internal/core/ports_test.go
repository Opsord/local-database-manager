package core

import (
	"net"
	"testing"
)

func TestIsPortAvailable(t *testing.T) {
	t.Parallel()

	// Dynamically bind to a port to make it occupied
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer listener.Close()

	occupiedPort := listener.Addr().(*net.TCPAddr).Port

	if IsPortAvailable(occupiedPort) {
		t.Errorf("expected port %d to be unavailable, but reported available", occupiedPort)
	}
}

func TestFindNextFreePort(t *testing.T) {
	t.Parallel()

	existing := []*DatabaseInstance{
		{Port: 5432},
		{Port: 5433},
	}

	freePort := FindNextFreePort(5432, existing)
	if freePort < 5434 {
		t.Errorf("expected free port >= 5434, got %d", freePort)
	}
}
