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
	exe, err := findDockerDesktopExe()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEngineStartFailed, err)
	}
	cmd := exec.Command(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: launch Docker Desktop: %v", ErrEngineStartFailed, err)
	}
	_ = cmd.Process.Release()
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()
	return r.waitUntilOnline(cmdCtx, "docker")
}

func (r *Runner) stopDockerEngine(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	// Best-effort graceful quit via Docker CLI (available while daemon is online).
	stopCmd := exec.CommandContext(cmdCtx, "docker", "desktop", "stop")
	stopCmd.Stdout = io.Discard
	stopCmd.Stderr = io.Discard
	_ = stopCmd.Run()

	// Fallback: launch Docker Desktop with --quit (documented quit flag).
	if exe, err := findDockerDesktopExe(); err == nil {
		quitCmd := exec.CommandContext(cmdCtx, exe, "--quit")
		if startErr := quitCmd.Start(); startErr == nil {
			_ = quitCmd.Process.Release()
		}
	}

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
