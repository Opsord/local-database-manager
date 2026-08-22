package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnvFile_Postgres(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "test_pg.env")

	content := `
# PostgreSQL Instance Config
ENGINE=postgres
RUNTIME=docker
CONTAINER_NAME=pg-test-calendar
COMPOSE_PROJECT_NAME=pg-test-calendar

POSTGRES_PORT=5433
POSTGRES_USER=myuser
POSTGRES_PASSWORD=secretpassword
POSTGRES_DB=calendar_db
POSTGRES_SCHEMA=custom_schema
POSTGRES_VOLUME=pgdata_test
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test env: %v", err)
	}

	inst, err := ParseEnvFile(envPath)
	if err != nil {
		t.Fatalf("ParseEnvFile failed: %v", err)
	}

	if inst.Name != "test_pg" {
		t.Errorf("expected Name 'test_pg', got '%s'", inst.Name)
	}
	if inst.EngineType != "postgres" {
		t.Errorf("expected EngineType 'postgres', got '%s'", inst.EngineType)
	}
	if inst.Runtime != "docker" {
		t.Errorf("expected Runtime 'docker', got '%s'", inst.Runtime)
	}
	if inst.ContainerName != "pg-test-calendar" {
		t.Errorf("expected ContainerName 'pg-test-calendar', got '%s'", inst.ContainerName)
	}
	if inst.Port != 5433 {
		t.Errorf("expected Port 5433, got %d", inst.Port)
	}
	if inst.User != "myuser" {
		t.Errorf("expected User 'myuser', got '%s'", inst.User)
	}
	if inst.Password != "secretpassword" {
		t.Errorf("expected Password 'secretpassword', got '%s'", inst.Password)
	}
	if inst.Database != "calendar_db" {
		t.Errorf("expected Database 'calendar_db', got '%s'", inst.Database)
	}
	if inst.Schema != "custom_schema" {
		t.Errorf("expected Schema 'custom_schema', got '%s'", inst.Schema)
	}

	expectedURI := "postgresql://myuser:secretpassword@localhost:5433/calendar_db?currentSchema=custom_schema"
	if uri := inst.ConnectionURI(); uri != expectedURI {
		t.Errorf("expected URI '%s', got '%s'", expectedURI, uri)
	}

	expectedCLI := "psql -h localhost -p 5433 -U myuser -d calendar_db"
	if cli := inst.CLICommand(); cli != expectedCLI {
		t.Errorf("expected CLI '%s', got '%s'", expectedCLI, cli)
	}

	envBlock := inst.BackendEnvBlock()
	if !strings.Contains(envBlock, "DB_HOST=localhost") || !strings.Contains(envBlock, "DATABASE_URL="+expectedURI) {
		t.Errorf("BackendEnvBlock failed: got '%s'", envBlock)
	}
}

func TestParseEnvFile_SQLServer(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "test_sql.env")

	content := `
ENGINE=sqlserver
RUNTIME=docker
CONTAINER_NAME=sql-test-app
COMPOSE_PROJECT_NAME=sql-test-app

SQLSERVER_PORT=1434
SA_PASSWORD=SuperSecretPass123!
SQLSERVER_DB=orders_db
SQLSERVER_SCHEMA=custom_dbo
SQLSERVER_VOLUME=sql_orders_data
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test env: %v", err)
	}

	inst, err := ParseEnvFile(envPath)
	if err != nil {
		t.Fatalf("ParseEnvFile failed: %v", err)
	}

	if inst.EngineType != "sqlserver" {
		t.Errorf("expected EngineType 'sqlserver', got '%s'", inst.EngineType)
	}
	if inst.Port != 1434 {
		t.Errorf("expected Port 1434, got %d", inst.Port)
	}
	if inst.User != "sa" {
		t.Errorf("expected User 'sa', got '%s'", inst.User)
	}
	if inst.Password != "SuperSecretPass123!" {
		t.Errorf("expected Password 'SuperSecretPass123!', got '%s'", inst.Password)
	}
	if inst.Database != "orders_db" {
		t.Errorf("expected Database 'orders_db', got '%s'", inst.Database)
	}
	if inst.Schema != "custom_dbo" {
		t.Errorf("expected Schema 'custom_dbo', got '%s'", inst.Schema)
	}

	expectedURI := "sqlserver://sa:SuperSecretPass123%21@localhost:1434?database=orders_db"
	if uri := inst.ConnectionURI(); uri != expectedURI {
		t.Errorf("expected URI '%s', got '%s'", expectedURI, uri)
	}

	envBlock := inst.BackendEnvBlock()
	if !strings.Contains(envBlock, "DB_PORT=1434") || !strings.Contains(envBlock, "DB_SCHEMA=custom_dbo") {
		t.Errorf("BackendEnvBlock failed: got '%s'", envBlock)
	}
}
