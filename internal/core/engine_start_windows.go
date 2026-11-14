//go:build windows

package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func launchDetachedProcess(file string, args string, showCmd int32) error {
	verbPtr := windows.StringToUTF16Ptr("open")
	filePtr := windows.StringToUTF16Ptr(file)
	var argsPtr *uint16
	if args != "" {
		argsPtr = windows.StringToUTF16Ptr(args)
	}
	return windows.ShellExecute(0, verbPtr, filePtr, argsPtr, nil, showCmd)
}

func startDockerDaemon(ctx context.Context, checkHealth func(context.Context) EngineHealth) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	var launchErr error
	exe, findErr := findDockerDesktopExe()
	if findErr == nil {
		launchErr = launchDetachedProcess(exe, "", windows.SW_SHOWNORMAL)
	} else {
		launchErr = findErr
	}

	if launchErr != nil {
		// Fallback to CLI detached launch with hidden window
		if cliErr := launchDetachedProcess("docker", "desktop start", windows.SW_HIDE); cliErr != nil {
			return fmt.Errorf("%w: launch Docker Desktop: %v (cli fallback: %v)", ErrEngineStartFailed, launchErr, cliErr)
		}
	}

	return waitUntilEngineOnline(cmdCtx, checkHealth, "docker")
}

func stopDockerDaemon(ctx context.Context, checkHealth func(context.Context) EngineHealth) error {
	cmdCtx, cancel := context.WithTimeout(ctx, engineStartTimeout)
	defer cancel()

	// Only use the Docker CLI. Launching "Docker Desktop.exe --quit" can spawn a
	// new Desktop process that reopens the tray/dashboard while the backend is
	// still shutting down.
	stopCmd := exec.CommandContext(cmdCtx, "docker", "desktop", "stop")
	stopCmd.Stdout = io.Discard
	stopCmd.Stderr = io.Discard
	_ = stopCmd.Run()

	return waitUntilEngineOffline(cmdCtx, checkHealth, "docker")
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
