package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/Opsord/local-database-manager/internal/config"
	"github.com/Opsord/local-database-manager/internal/core"
	"github.com/Opsord/local-database-manager/internal/updater"
	"github.com/Opsord/local-database-manager/internal/version"
)

// Run executes a CLI subcommand and returns the process exit code.
func Run(projectRoot string, cfg config.Config, args []string) int {
	return RunWithOutput(projectRoot, cfg, args, os.Stdout, os.Stderr)
}

// RunWithOutput allows dependency injection of stdout/stderr for testing.
func RunWithOutput(projectRoot string, cfg config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	instancesDir := filepath.Join(projectRoot, "instances")
	runner := core.NewRunner(projectRoot)

	subcmd := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subcmd {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0

	case "list", "ls":
		return runList(instancesDir, runner, stdout, stderr)

	case "up", "start":
		return runStart(instancesDir, runner, subArgs, stdout, stderr)

	case "down", "stop":
		return runStop(instancesDir, runner, subArgs, stdout, stderr)

	case "restart":
		return runRestart(instancesDir, runner, subArgs, stdout, stderr)

	case "uri":
		return runURI(instancesDir, subArgs, stdout, stderr)

	case "env":
		return runEnv(instancesDir, subArgs, stdout, stderr)

	case "logs":
		return runLogs(instancesDir, runner, subArgs, stdout, stderr)

	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "Local Database Manager %s (commit: %s, built: %s)\n", version.Display(), version.Commit, version.Date)
		return 0

	case "update":
		return runUpdate(cfg, subArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "Unknown command: %q\n\n", subcmd)
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Local Database Manager (dbm) - Headless CLI

Usage:
  dbm [command] [arguments...]

Commands:
  list, ls                 List all configured database instances and statuses
  up, start <name>         Start a database instance
  down, stop <name>        Stop a running database instance
  restart <name>           Restart a database instance
  uri <name> [-c|--copy]   Print or copy connection URI
  env <name> [-c|--copy]   Print or copy backend .env configuration block
  logs <name>              Stream live container logs
  update [--check]         Check for or install application updates
  version, -v, --version   Show version information
  help, -h, --help         Show this help message

Run 'dbm' without arguments to launch the interactive Terminal User Interface (TUI).`)
}

func loadInstances(instancesDir string) ([]*core.DatabaseInstance, error) {
	return core.ScanInstances(instancesDir)
}

func findInstance(instances []*core.DatabaseInstance, name string) *core.DatabaseInstance {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, inst := range instances {
		if strings.ToLower(inst.Name) == name || strings.ToLower(inst.ID) == name {
			return inst
		}
	}
	return nil
}

func runList(instancesDir string, runner core.InstanceRunner, stdout, stderr io.Writer) int {
	instances, err := loadInstances(instancesDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading instances: %v\n", err)
		return 1
	}

	if len(instances) == 0 {
		fmt.Fprintln(stdout, "No database instances configured in instances/")
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, inst := range instances {
		wg.Add(1)
		go func(i *core.DatabaseInstance) {
			defer wg.Done()
			i.Status = runner.CheckStatus(ctx, i)
			if i.Status != core.StatusStopped {
				stats := runner.GetStats(ctx, i)
				i.CPUUsage = stats.CPU
				i.MemoryUsage = stats.Memory
			} else {
				i.CPUUsage = "-"
				i.MemoryUsage = "-"
			}
		}(inst)
	}
	wg.Wait()

	w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tENGINE\tRUNTIME\tPORT\tSTATUS\tCPU\tMEMORY\tDATABASE")
	for _, i := range instances {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			i.Name,
			i.EngineType,
			i.Runtime,
			i.Port,
			i.Status,
			i.CPUUsage,
			i.MemoryUsage,
			i.Database,
		)
	}
	_ = w.Flush()
	return 0
}

func runStart(instancesDir string, runner core.InstanceRunner, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: dbm up <instance_name>")
		return 1
	}
	instName := args[0]
	instances, err := loadInstances(instancesDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading instances: %v\n", err)
		return 1
	}
	inst := findInstance(instances, instName)
	if inst == nil {
		fmt.Fprintf(stderr, "Error: Instance %q not found in instances/\n", instName)
		return 1
	}

	ctx := context.Background()
	currentStatus := runner.CheckStatus(ctx, inst)
	if currentStatus == core.StatusReady {
		fmt.Fprintf(stdout, "Instance %q is already running and ready on port %d.\n", inst.Name, inst.Port)
		return 0
	}

	fmt.Fprintf(stdout, "Starting %s (%s)... ", inst.Name, inst.ContainerName)
	if err := runner.Start(ctx, inst); err != nil {
		fmt.Fprintf(stderr, "\nFailed to start: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "Started.")

	status := runner.CheckStatus(ctx, inst)
	fmt.Fprintf(stdout, "Status: %s (Port: %d)\n", status, inst.Port)
	fmt.Fprintf(stdout, "URI: %s\n", inst.ConnectionURI())
	return 0
}

func runStop(instancesDir string, runner core.InstanceRunner, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: dbm down <instance_name>")
		return 1
	}
	instName := args[0]
	instances, err := loadInstances(instancesDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading instances: %v\n", err)
		return 1
	}
	inst := findInstance(instances, instName)
	if inst == nil {
		fmt.Fprintf(stderr, "Error: Instance %q not found in instances/\n", instName)
		return 1
	}

	ctx := context.Background()
	fmt.Fprintf(stdout, "Stopping %s (%s)... ", inst.Name, inst.ContainerName)
	if err := runner.Stop(ctx, inst); err != nil {
		fmt.Fprintf(stderr, "\nFailed to stop: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "Stopped.")
	return 0
}

func runRestart(instancesDir string, runner core.InstanceRunner, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: dbm restart <instance_name>")
		return 1
	}
	instName := args[0]
	instances, err := loadInstances(instancesDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading instances: %v\n", err)
		return 1
	}
	inst := findInstance(instances, instName)
	if inst == nil {
		fmt.Fprintf(stderr, "Error: Instance %q not found in instances/\n", instName)
		return 1
	}

	ctx := context.Background()
	fmt.Fprintf(stdout, "Restarting %s...\n", inst.Name)
	_ = runner.Stop(ctx, inst)
	if err := runner.Start(ctx, inst); err != nil {
		fmt.Fprintf(stderr, "Failed to start: %v\n", err)
		return 1
	}
	status := runner.CheckStatus(ctx, inst)
	fmt.Fprintf(stdout, "Instance %q restarted successfully. Status: %s (Port: %d)\n", inst.Name, status, inst.Port)
	return 0
}

func runURI(instancesDir string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: dbm uri <instance_name> [-c|--copy]")
		return 1
	}
	instName := args[0]
	shouldCopy := hasCopyFlag(args[1:])

	instances, err := loadInstances(instancesDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading instances: %v\n", err)
		return 1
	}
	inst := findInstance(instances, instName)
	if inst == nil {
		fmt.Fprintf(stderr, "Error: Instance %q not found in instances/\n", instName)
		return 1
	}

	uri := inst.ConnectionURI()
	if shouldCopy {
		if err := core.CopyToClipboard(uri); err != nil {
			fmt.Fprintf(stderr, "Error copying to clipboard: %v\n", err)
			fmt.Fprintln(stdout, uri)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n(Copied to clipboard)\n", uri)
	} else {
		fmt.Fprintln(stdout, uri)
	}
	return 0
}

func runEnv(instancesDir string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: dbm env <instance_name> [-c|--copy]")
		return 1
	}
	instName := args[0]
	shouldCopy := hasCopyFlag(args[1:])

	instances, err := loadInstances(instancesDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading instances: %v\n", err)
		return 1
	}
	inst := findInstance(instances, instName)
	if inst == nil {
		fmt.Fprintf(stderr, "Error: Instance %q not found in instances/\n", instName)
		return 1
	}

	block := inst.BackendEnvBlock()
	if shouldCopy {
		if err := core.CopyToClipboard(block); err != nil {
			fmt.Fprintf(stderr, "Error copying to clipboard: %v\n", err)
			fmt.Fprintln(stdout, block)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n\n(Copied to clipboard)\n", block)
	} else {
		fmt.Fprintln(stdout, block)
	}
	return 0
}

func runLogs(instancesDir string, runner core.InstanceRunner, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: dbm logs <instance_name>")
		return 1
	}
	instName := args[0]

	instances, err := loadInstances(instancesDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error loading instances: %v\n", err)
		return 1
	}
	inst := findInstance(instances, instName)
	if inst == nil {
		fmt.Fprintf(stderr, "Error: Instance %q not found in instances/\n", instName)
		return 1
	}

	cmd := runner.LogsCommand(inst)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "Logs stream ended: %v\n", err)
		return 1
	}
	return 0
}

func hasCopyFlag(args []string) bool {
	for _, a := range args {
		if a == "-c" || a == "--copy" {
			return true
		}
	}
	return false
}

func runUpdate(cfg config.Config, args []string, stdout, stderr io.Writer) int {
	checkOnly := false
	for _, a := range args {
		switch a {
		case "--check", "-c":
			checkOnly = true
		default:
			fmt.Fprintf(stderr, "Unknown flag for update: %q\nUsage: dbm update [--check]\n", a)
			return 1
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fmt.Fprintln(stdout, "Checking for updates...")
	info, err := updater.Check(ctx, updater.CheckOptions{
		Target:            updater.TargetCLI,
		IncludePrerelease: cfg.UpdateChannel == "prerelease",
	})
	if err != nil {
		fmt.Fprintf(stderr, "Error checking for updates: %v\n", err)
		return 1
	}

	if !info.UpdateAvailable {
		fmt.Fprintf(stdout, "Local Database Manager is up to date (%s).\n", info.CurrentVersion)
		return 0
	}

	fmt.Fprintf(stdout, "Update available: %s -> %s (%s)\n", info.CurrentVersion, info.LatestVersion, info.ReleaseURL)
	if checkOnly {
		return 0
	}

	applyCtx, applyCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer applyCancel()

	lastStage := updater.ProgressStage("")
	err = updater.Apply(applyCtx, info, updater.ApplyOptions{
		OnProgress: func(stage updater.ProgressStage, pct int, msg string) {
			if stage != lastStage || pct == 100 || pct%25 == 0 {
				lastStage = stage
				fmt.Fprintf(stdout, "  %s\n", msg)
			}
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "Error applying update: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Successfully updated to %s! Restart db-manager to use the new version.\n", info.LatestVersion)
	return 0
}
