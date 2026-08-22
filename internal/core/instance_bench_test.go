package core

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func BenchmarkParseEnvFile(b *testing.B) {
	tmpDir := b.TempDir()
	envPath := filepath.Join(tmpDir, "bench.env")
	content := `ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-bench
COMPOSE_PROJECT_NAME=pg-bench
MEMORY_LIMIT=512M
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=bench_db
POSTGRES_SCHEMA=public
POSTGRES_VOLUME=pgdata_bench
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write bench env: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParseEnvFile(envPath)
		if err != nil {
			b.Fatalf("ParseEnvFile failed: %v", err)
		}
	}
}

func BenchmarkFindNextFreePort(b *testing.B) {
	var instances []*DatabaseInstance
	for p := 5432; p < 5442; p++ {
		instances = append(instances, &DatabaseInstance{
			Port: p,
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = FindNextFreePort(5432, instances)
	}
}

func BenchmarkScanInstances(b *testing.B) {
	tmpDir := b.TempDir()
	for i := 0; i < 10; i++ {
		path := filepath.Join(tmpDir, "inst_"+strconv.Itoa(i)+".env")
		_ = os.WriteFile(path, []byte("ENGINE=postgres\nPOSTGRES_PORT=5432\nPOSTGRES_DB=mydb\n"), 0644)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = ScanInstances(tmpDir)
	}
}
