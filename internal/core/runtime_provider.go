package core

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// RuntimeProvider defines the operations that a container engine (Docker, Podman) must implement.
type RuntimeProvider interface {
	Name() string
	Health(ctx context.Context) EngineHealth
	StartDaemon(ctx context.Context) error
	StopDaemon(ctx context.Context) error
	Start(ctx context.Context, inst *DatabaseInstance) error
	Stop(ctx context.Context, inst *DatabaseInstance) error
	DownVolumes(ctx context.Context, inst *DatabaseInstance) error
	CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus
	GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string
	LogsCommand(inst *DatabaseInstance) *exec.Cmd
}

// waitUntilEngineOnline polls until the specified engine becomes online or times out.
func waitUntilEngineOnline(ctx context.Context, checkHealth func(context.Context) EngineHealth, engineName string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, engineStartTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if checkHealth(ctx) == EngineOnline {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("%w: timed out waiting for %s", ErrEngineStartFailed, engineName)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: timed out waiting for %s", ErrEngineStartFailed, engineName)
		case <-time.After(enginePollInterval):
		}
	}
}

// waitUntilEngineOffline polls until the specified engine becomes offline or times out.
func waitUntilEngineOffline(ctx context.Context, checkHealth func(context.Context) EngineHealth, engineName string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, engineStartTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}
	for {
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("%w: timed out waiting for %s to stop", ErrEngineStartFailed, engineName)
		}
		if checkHealth(ctx) != EngineOnline {
			if ctx.Err() != nil {
				return fmt.Errorf("%w: timed out waiting for %s to stop", ErrEngineStartFailed, engineName)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: timed out waiting for %s to stop", ErrEngineStartFailed, engineName)
		case <-time.After(enginePollInterval):
		}
	}
}

// unsupportedProvider handles unknown or unregistered container runtimes.
type unsupportedProvider struct {
	name string
}

func newUnsupportedProvider(name string) *unsupportedProvider {
	return &unsupportedProvider{name: name}
}

func (u *unsupportedProvider) Name() string {
	return u.name
}

func (u *unsupportedProvider) Health(ctx context.Context) EngineHealth {
	bin := strings.TrimSpace(u.name)
	if bin == "" {
		return EngineNotInstalled
	}
	if _, err := exec.LookPath(bin); err != nil {
		return EngineNotInstalled
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, bin, "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return EngineOffline
	}
	return EngineOnline
}

func (u *unsupportedProvider) StartDaemon(ctx context.Context) error {
	h := u.Health(ctx)
	if h == EngineNotInstalled {
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, u.name)
	}
	if h == EngineOnline {
		return nil
	}
	return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, u.name)
}

func (u *unsupportedProvider) StopDaemon(ctx context.Context) error {
	h := u.Health(ctx)
	if h == EngineNotInstalled {
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, u.name)
	}
	if h == EngineOffline {
		return nil
	}
	return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, u.name)
}

func (u *unsupportedProvider) Start(ctx context.Context, inst *DatabaseInstance) error {
	h := u.Health(ctx)
	if h == EngineNotInstalled {
		return fmt.Errorf("%w: %s", ErrEngineNotInstalled, u.name)
	}
	if h == EngineOffline {
		return fmt.Errorf("%w: %s daemon is not running", ErrEngineOffline, u.name)
	}
	return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, u.name)
}

func (u *unsupportedProvider) Stop(ctx context.Context, inst *DatabaseInstance) error {
	return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, u.name)
}

func (u *unsupportedProvider) DownVolumes(ctx context.Context, inst *DatabaseInstance) error {
	return fmt.Errorf("%w: unsupported runtime %q", ErrEngineStartFailed, u.name)
}

func (u *unsupportedProvider) CheckStatus(ctx context.Context, inst *DatabaseInstance) ContainerStatus {
	return StatusStopped
}

func (u *unsupportedProvider) GetMemoryUsage(ctx context.Context, inst *DatabaseInstance) string {
	return "-"
}

func (u *unsupportedProvider) LogsCommand(inst *DatabaseInstance) *exec.Cmd {
	return exec.Command(u.name, "logs", inst.ContainerName)
}
