//go:build windows

package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func (r *Runner) startDockerEngine(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	// Prefer CLI start (less UI than launching the .exe). Fall back to the
	// Desktop executable when the CLI plugin is missing or fails to start.
	startCmd := exec.CommandContext(cmdCtx, "docker", "desktop", "start")
	startCmd.Stdout = io.Discard
	startCmd.Stderr = io.Discard
	if err := startCmd.Run(); err != nil {
		if r.CheckEngineHealth(cmdCtx, "docker") == EngineOnline {
			return nil
		}
		exe, findErr := findDockerDesktopExe()
		if findErr != nil {
			return fmt.Errorf("%w: docker desktop start: %v (%v)", ErrEngineStartFailed, err, findErr)
		}
		cmd := exec.Command(exe)
		if startErr := cmd.Start(); startErr != nil {
			return fmt.Errorf("%w: launch Docker Desktop: %v (cli start: %v)", ErrEngineStartFailed, startErr, err)
		}
		_ = cmd.Process.Release()
	}
	return r.waitUntilOnline(cmdCtx, "docker")
}

func (r *Runner) stopDockerEngine(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	// Only use the Docker CLI. Launching "Docker Desktop.exe --quit" can spawn a
	// new Desktop process that reopens the tray/dashboard while the backend is
	// still shutting down.
	stopCmd := exec.CommandContext(cmdCtx, "docker", "desktop", "stop")
	stopCmd.Stdout = io.Discard
	stopCmd.Stderr = io.Discard
	_ = stopCmd.Run()

	return r.waitUntilOffline(cmdCtx, "docker")
}

func findDockerDesktopExe() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Docker", "Docker", "Docker Desktop.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Docker", "Docker", "Docker Desktop.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Docker", "Docker Desktop.exe"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("Docker Desktop.exe"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("Docker Desktop.exe not found")
}
