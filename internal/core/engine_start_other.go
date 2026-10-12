//go:build !windows

package core

import (
	"context"
	"fmt"
)

func startDockerDaemon(ctx context.Context, checkHealth func(context.Context) EngineHealth) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()
	if err := waitUntilEngineOnline(cmdCtx, checkHealth, "docker"); err != nil {
		return fmt.Errorf("%w: docker is offline (auto-start supported on Windows via Docker Desktop)", ErrEngineStartFailed)
	}
	return nil
}

func stopDockerDaemon(ctx context.Context, checkHealth func(context.Context) EngineHealth) error {
	_ = ctx
	_ = checkHealth
	return fmt.Errorf("%w: docker is online (auto-stop supported on Windows via Docker Desktop)", ErrEngineStartFailed)
}

func launchDetachedProcess(file string, args string, showCmd int32) error {
	_ = file
	_ = args
	_ = showCmd
	return fmt.Errorf("launchDetachedProcess: only supported on Windows")
}
