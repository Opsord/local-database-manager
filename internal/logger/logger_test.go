package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerInitAndWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbm_log_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	closeFn, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	defer closeFn()

	slog.Info("test info message", "key", "value123")
	slog.Error("test error message", "code", "ERR_CODE")

	// Close the log file so we can read it on Windows
	closeFn()

	logPath := filepath.Join(tmpDir, LogFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "test info message") || !strings.Contains(strContent, "value123") {
		t.Errorf("log missing info message, got: %s", strContent)
	}
	if !strings.Contains(strContent, "test error message") || !strings.Contains(strContent, "ERR_CODE") {
		t.Errorf("log missing error message, got: %s", strContent)
	}
}
