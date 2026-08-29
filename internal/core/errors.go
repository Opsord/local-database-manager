package core

import "errors"

var (
	// ErrEngineOffline is returned when the container engine daemon is unreachable.
	ErrEngineOffline = errors.New("container engine daemon is offline")

	// ErrEngineNotInstalled is returned when docker/podman is not found in PATH.
	ErrEngineNotInstalled = errors.New("container engine not found in system PATH")

	// ErrEngineStartFailed is returned when the container engine could not be started.
	ErrEngineStartFailed = errors.New("failed to start container engine")

	// ErrPortInUse is returned when a port is not available.
	ErrPortInUse = errors.New("requested port is already in use")

	// ErrInstanceNotFound is returned when an instance cannot be located.
	ErrInstanceNotFound = errors.New("database instance not found")
)
