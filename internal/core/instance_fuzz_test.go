package core

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseEnvFile(f *testing.F) {
	// Seed corpus with valid, partial, and edge-case configs
	f.Add([]byte("ENGINE=postgres\nPOSTGRES_PORT=5432\nPOSTGRES_DB=testdb\n"))
	f.Add([]byte("ENGINE=sqlserver\nSQLSERVER_PORT=1433\nSA_PASSWORD=Secret123!\n"))
	f.Add([]byte("# Comment only\nKEY=value # inline comment\nQUOTED=\"hello world\"\n"))
	f.Add([]byte("EMPTY=\nINVALID LINE WITHOUT EQUALS\n===EXTRA===\n"))
	f.Add([]byte("ENGINE=unknown\nPORT=9999\nKEY='single quote'\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		tmpDir := t.TempDir()
		envPath := filepath.Join(tmpDir, "fuzz.env")
		if err := os.WriteFile(envPath, data, 0644); err != nil {
			return
		}

		// ParseEnvFile must be resilient and never panic on arbitrary input
		inst, err := ParseEnvFile(envPath)
		if err == nil && inst != nil {
			_ = inst.ConnectionURI()
			_ = inst.CLICommand()
			_ = inst.BackendEnvBlock()
		}
	})
}
