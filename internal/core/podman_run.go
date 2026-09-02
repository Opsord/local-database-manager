package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// buildPodmanRunArgs builds `podman run` args for an instance.
// Used instead of `podman compose` on Windows: external docker-compose talks to the
// rootful named-pipe API where netavark/nftables often fails, while rootless
// `podman run` (pasta) works.
func (r *Runner) buildPodmanRunArgs(inst *DatabaseInstance) ([]string, error) {
	name := strings.TrimSpace(inst.ContainerName)
	if name == "" {
		return nil, fmt.Errorf("container name is required")
	}
	if inst.Port <= 0 {
		return nil, fmt.Errorf("host port is required")
	}

	args := []string{
		"run", "-d", "--replace",
		"--name", name,
		"--restart", "unless-stopped",
	}
	if mem := strings.TrimSpace(inst.MemoryLimit); mem != "" {
		args = append(args, "--memory", mem)
	}

	switch inst.EngineType {
	case "sqlserver":
		vol := strings.TrimSpace(inst.Volume)
		if vol == "" {
			vol = "mssqldata"
		}
		engineDir := filepath.Join(r.ProjectRoot, "engines", "sql-server")
		args = append(args,
			"-e", "ACCEPT_EULA=Y",
			"-e", "MSSQL_PID=Developer",
			"-e", "MSSQL_SA_PASSWORD="+inst.Password,
			"-e", "SA_PASSWORD="+inst.Password,
			"-e", "SQLSERVER_DB="+inst.Database,
			"-e", "SQLSERVER_SCHEMA="+inst.Schema,
			"-p", fmt.Sprintf("127.0.0.1:%d:1433", inst.Port),
			"-v", vol+":/var/opt/mssql",
			"-v", filepath.Join(engineDir, "init")+":/init:ro",
			"-v", filepath.Join(engineDir, "docker-entrypoint.sh")+":/entrypoint.sh:ro",
			"--entrypoint", "/bin/bash",
			"mcr.microsoft.com/mssql/server:2022-latest",
			"/entrypoint.sh",
		)
		if inst.RawEnv != nil {
			if mb := strings.TrimSpace(inst.RawEnv["MSSQL_MEMORY_LIMIT_MB"]); mb != "" {
				args = insertEnvArg(args, "MSSQL_MEMORY_LIMIT_MB="+mb)
			}
		}
	default: // postgres
		version := strings.TrimSpace(inst.Version)
		if version == "" {
			version = "18"
		}
		vol := strings.TrimSpace(inst.Volume)
		if vol == "" {
			vol = "pgdata"
		}
		initDir := filepath.Join(r.ProjectRoot, "engines", "postgres", "init")
		args = append(args,
			"-e", "POSTGRES_DB="+inst.Database,
			"-e", "POSTGRES_USER="+inst.User,
			"-e", "POSTGRES_PASSWORD="+inst.Password,
			"-e", "POSTGRES_SCHEMA="+inst.Schema,
			"-p", fmt.Sprintf("127.0.0.1:%d:5432", inst.Port),
			"-v", vol+":/var/lib/postgresql",
			"-v", initDir+":/docker-entrypoint-initdb.d:ro",
			"docker.io/library/postgres:"+version,
		)
	}
	return args, nil
}

func insertEnvArg(args []string, envKV string) []string {
	// Place before the image token (last non-entrypoint arg that looks like image).
	out := make([]string, 0, len(args)+2)
	inserted := false
	for i, a := range args {
		if !inserted && (strings.HasPrefix(a, "docker.io/") || strings.HasPrefix(a, "mcr.microsoft.com/")) {
			out = append(out, "-e", envKV)
			inserted = true
		}
		_ = i
		out = append(out, a)
	}
	if !inserted {
		out = append(out, "-e", envKV)
	}
	return out
}

func (r *Runner) startPodmanContainer(ctx context.Context, inst *DatabaseInstance) error {
	// Compose via the Windows named pipe creates rootful containers. A leftover
	// "Created" rootful container with the same name/port can break host publish.
	r.removeStaleRootfulContainer(ctx, inst.ContainerName)

	args, err := r.buildPodmanRunArgs(inst)
	if err != nil {
		return err
	}
	// #region agent log
	AgentDebugLog("J", "runner.go:startPodmanContainer", "run", map[string]any{
		"name": inst.ContainerName, "engine": inst.EngineType, "port": inst.Port, "runId": "post-fix",
	})
	// #endregion
	cmd := exec.CommandContext(ctx, "podman", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		detail := FormatComposeStderr(errBuf.String())
		if detail == "" {
			detail = err.Error()
		}
		// #region agent log
		AgentDebugLog("J", "runner.go:startPodmanContainer", "run_failed", map[string]any{
			"detail": detail, "runId": "post-fix",
		})
		// #endregion
		return fmt.Errorf("failed to start container: %s (%w)", detail, err)
	}
	return nil
}

// removeStaleRootfulContainer best-effort deletes a same-named container on the
// Docker-API named pipe (rootful), which is invisible to rootless `podman ps`.
func (r *Runner) removeStaleRootfulContainer(ctx context.Context, name string) {
	if runtime.GOOS != "windows" || strings.TrimSpace(name) == "" {
		return
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return
	}
	host, err := r.podmanMachineDOCKERHost(ctx)
	if err != nil || host == "" {
		return
	}
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
	cmd.Env = append(os.Environ(), "DOCKER_HOST="+host)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	// #region agent log
	AgentDebugLog("J", "runner.go:removeStaleRootfulContainer", "attempted", map[string]any{
		"name": name, "host": host, "runId": "post-fix",
	})
	// #endregion
}

func (r *Runner) stopPodmanContainer(ctx context.Context, inst *DatabaseInstance) error {
	cmd := exec.CommandContext(ctx, "podman", "rm", "-f", inst.ContainerName)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errBuf.String())
		lower := strings.ToLower(detail + " " + err.Error())
		if strings.Contains(lower, "no such container") || strings.Contains(lower, "not found") {
			return nil
		}
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("failed to stop container: %s (%w)", detail, err)
	}
	return nil
}

func (r *Runner) purgePodmanContainer(ctx context.Context, inst *DatabaseInstance) error {
	if err := r.stopPodmanContainer(ctx, inst); err != nil {
		return fmt.Errorf("failed to purge container: %w", err)
	}
	vol := strings.TrimSpace(inst.Volume)
	if vol == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "podman", "volume", "rm", "-f", vol)
	_ = cmd.Run() // volume may already be gone
	return nil
}
