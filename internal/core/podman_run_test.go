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
