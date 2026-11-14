package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// LogFileName is the standard log file name in the project root.
const LogFileName = ".db-manager.log"

var (
	mu      sync.Mutex
	logFile *os.File
)

// Init initializes the default slog logger to write structured JSON logs
// to .db-manager.log in projectRoot.
// It returns a cleanup function to safely sync and close the log file.
func Init(projectRoot string) (func(), error) {
	mu.Lock()
	defer mu.Unlock()

	logPath := filepath.Join(projectRoot, LogFileName)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return func() {}, fmt.Errorf("open log file %q: %w", logPath, err)
	}

	logFile = f
	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))

	closeFn := func() {
		mu.Lock()
		defer mu.Unlock()
		if logFile != nil {
			_ = logFile.Sync()
			_ = logFile.Close()
			logFile = nil
		}
	}

	return closeFn, nil
}
