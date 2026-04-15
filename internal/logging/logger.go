// Package logging provides a process-wide structured logger that writes to
// ~/.ado/logs/ado-YYYY-MM-DD.log. It is safe to call L() from any goroutine;
// the logger is initialized lazily on first use.
//
// This package exists so that CLI commands (dispatched through the mediator)
// and the TUI (which bypasses the mediator and calls the API client directly)
// can share the same log file, giving users a single place to tail for
// runtime activity regardless of which entry point they used.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	once    sync.Once
	logger  *slog.Logger
	file    *os.File
	initErr error
)

// L returns the shared ado logger, initializing it on first use. If
// initialization fails (e.g. home dir unavailable, disk full), L returns a
// logger that discards everything; the underlying error is available via Err.
func L() *slog.Logger {
	once.Do(initLogger)
	if logger == nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return logger
}

// Err returns the error (if any) encountered while initializing the logger.
func Err() error { return initErr }

// Close releases the log file handle. Safe to call even if the logger was
// never initialized.
func Close() error {
	if file == nil {
		return nil
	}
	err := file.Close()
	file = nil
	return err
}

func initLogger() {
	home, err := os.UserHomeDir()
	if err != nil {
		initErr = fmt.Errorf("resolve home dir: %w", err)
		return
	}
	dir := filepath.Join(home, ".ado", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		initErr = fmt.Errorf("create log dir %s: %w", dir, err)
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("ado-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		initErr = fmt.Errorf("open log file %s: %w", path, err)
		return
	}
	file = f
	logger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
