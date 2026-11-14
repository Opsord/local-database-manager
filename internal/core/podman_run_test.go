package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPodmanRunArgs_Postgres(t *testing.T) {
	t.Parallel()
	r := NewRunner(`C:\proj`)
	inst := &DatabaseInstance{
		EngineType:    "postgres",
		Runtime:       "podman",
		ContainerName: "pg_super_calendad_db",
		Port:          5434,
		Database:      "super_calendad_db",
		User:          "postgres",
		Password:      "postgres",
		Schema:        "public",
		Version:       "18",
		Volume:        "pgdata_sc_db_18",
		MemoryLimit:   "512M",
	}
	args, err := r.buildPodmanRunArgs(inst)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"run -d --replace --name pg_super_calendad_db",
		"--memory 512M",
		"-p 127.0.0.1:5434:5432",
		"pgdata_sc_db_18:/var/lib/postgresql",
		filepath.Join(`C:\proj`, "engines", "postgres", "init") + ":/docker-entrypoint-initdb.d:ro",
		"docker.io/library/postgres:18",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
	if strings.Contains(joined, "compose") {
		t.Fatal("must not use compose")
	}
}

func TestBuildPodmanRunArgs_SQLServer_AzureEdge(t *testing.T) {
	t.Parallel()
	r := NewRunner(`C:\proj`)
	inst := &DatabaseInstance{
		EngineType:    "sqlserver",
		Runtime:       "podman",
		ContainerName: "sql_edge_test",
		Port:          1433,
		Database:      "edge_db",
		Password:      "SuperSecretPass123!",
		Schema:        "dbo",
		Version:       SQLServerAzureEdge,
		Volume:        "sqlserver_edge",
		MemoryLimit:   "1G",
	}
	args, err := r.buildPodmanRunArgs(inst)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"run -d --replace --name sql_edge_test",
		"--memory 1G",
		"-p 127.0.0.1:1433:1433",
		"sqlserver_edge:/var/opt/mssql",
		"mcr.microsoft.com/azure-sql-edge:latest",
		"/entrypoint.sh",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
}

func TestBuildPodmanRunArgs_SQLServer_2022(t *testing.T) {
	t.Parallel()
	r := NewRunner(`C:\proj`)
	inst := &DatabaseInstance{
		EngineType:    "sqlserver",
		Runtime:       "podman",
		ContainerName: "sql_2022_test",
		Port:          1433,
		Database:      "orders_db",
		Password:      "SuperSecretPass123!",
		Schema:        "dbo",
		Version:       SQLServer2022,
		Volume:        "sqlserver_orders",
		MemoryLimit:   "2G",
	}
	args, err := r.buildPodmanRunArgs(inst)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"run -d --replace --name sql_2022_test",
		"--memory 2G",
		"-p 127.0.0.1:1433:1433",
		"sqlserver_orders:/var/opt/mssql",
		"mcr.microsoft.com/mssql/server:2022-latest",
		"/entrypoint.sh",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
}

func TestRemoveFlagWithValue(t *testing.T) {
	t.Parallel()
	args := []string{"run", "-d", "--name", "test", "--memory", "512M", "-p", "1433:1433"}
	cleaned := removeFlagWithValue(args, "--memory")
	joined := strings.Join(cleaned, " ")
	if strings.Contains(joined, "--memory") || strings.Contains(joined, "512M") {
		t.Fatalf("flag was not removed: %v", cleaned)
	}
	if len(cleaned) != 6 {
		t.Fatalf("expected len 6, got %d (%v)", len(cleaned), cleaned)
	}
}

func TestParseMemoryMB(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"512M", "512"},
		{"512MB", "512"},
		{"1G", "1024"},
		{"2GB", "2048"},
		{"", ""},
		{"invalid", ""},
	}
	for _, tc := range cases {
		if got := parseMemoryMB(tc.in); got != tc.want {
			t.Fatalf("parseMemoryMB(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPodmanProvider_CgroupMemoryFlagRemoval(t *testing.T) {
	t.Parallel()
	p := NewPodmanProvider(".")
	inst := &DatabaseInstance{
		EngineType:    "sqlserver",
		Runtime:       "podman",
		ContainerName: "sql_edge_test",
		Port:          1433,
		Database:      "edge_db",
		Password:      "SuperSecretPass123!",
		Schema:        "dbo",
		Version:       SQLServerAzureEdge,
		Volume:        "sqlserver_edge",
		MemoryLimit:   "512M",
	}
	args, err := p.buildPodmanRunArgs(inst)
	if err != nil {
		t.Fatal(err)
	}
	// Verify --memory is originally generated
	if !strings.Contains(strings.Join(args, " "), "--memory 512M") {
		t.Fatalf("expected --memory 512M in args: %v", args)
	}

	// Verify MSSQL_MEMORY_LIMIT_MB is derived from MemoryLimit
	if !strings.Contains(strings.Join(args, " "), "MSSQL_MEMORY_LIMIT_MB=512") {
		t.Fatalf("expected MSSQL_MEMORY_LIMIT_MB=512 in args: %v", args)
	}

	// Strip memory flag when host does not support cgroup memory
	cleaned := removeFlagWithValue(args, "--memory")
	if strings.Contains(strings.Join(cleaned, " "), "--memory") {
		t.Fatalf("--memory should be removed: %v", cleaned)
	}
}
