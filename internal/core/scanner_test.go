package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanInstances(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	env1 := filepath.Join(tmpDir, "super_calendar.env")
	_ = os.WriteFile(env1, []byte("ENGINE=postgres\nPOSTGRES_DB=calendar\nPOSTGRES_PORT=5432\n"), 0644)

	env2 := filepath.Join(tmpDir, "requerimientos.env")
	_ = os.WriteFile(env2, []byte("ENGINE=sqlserver\nSQLSERVER_DB=reqs\nSQLSERVER_PORT=1433\n"), 0644)

	template := filepath.Join(tmpDir, ".env.template")
	_ = os.WriteFile(template, []byte("ENGINE=postgres\n"), 0644)

	instances, err := ScanInstances(tmpDir)
	if err != nil {
		t.Fatalf("ScanInstances failed: %v", err)
	}

	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	// Should be sorted alphabetically: requerimientos, super_calendar
	if instances[0].Name != "requerimientos" {
		t.Errorf("expected first instance 'requerimientos', got '%s'", instances[0].Name)
	}
	if instances[1].Name != "super_calendar" {
		t.Errorf("expected second instance 'super_calendar', got '%s'", instances[1].Name)
	}
}
