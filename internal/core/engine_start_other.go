//go:build !windows

package core

import (
	"context"
	"fmt"
)

func (r *Runner) startDockerEngine(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()
	if err := r.waitUntilOnline(cmdCtx, "docker"); err != nil {
		return fmt.Errorf("%w: docker is offline (auto-start supported on Windows via Docker Desktop)", ErrEngineStartFailed)
	}
	return nil
}

func (r *Runner) stopDockerEngine(ctx context.Context) error {
	_ = ctx
	return fmt.Errorf("%w: docker is online (auto-stop supported on Windows via Docker Desktop)", ErrEngineStartFailed)
}
