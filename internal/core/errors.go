package core

import (
	"errors"
	"fmt"
)

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

// Standard error codes for domain errors.
const (
	ErrCodePortInUse          = "PORT_IN_USE"
	ErrCodeEngineOffline      = "ENGINE_OFFLINE"
	ErrCodeEngineNotInstalled  = "ENGINE_NOT_INSTALLED"
	ErrCodeEngineStartFailed   = "ENGINE_START_FAILED"
	ErrCodeComposeFailed      = "COMPOSE_FAILED"
	ErrCodeInstanceNotFound   = "INSTANCE_NOT_FOUND"
)

// AppError is an enriched error that carries an error code, message,
// an optional actionable remediation suggestion for the UI, and the underlying error cause.
type AppError struct {
	Code       string
	Message    string
	Suggestion string
	Err        error
}

func (e *AppError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s — Tip: %s", e.Message, e.Suggestion)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError instantiates a new AppError with code, message, suggestion, and cause.
func NewAppError(code, message, suggestion string, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Suggestion: suggestion,
		Err:        err,
	}
}
