//go:build windows

package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxLogSize  = 10 * 1024 * 1024 // 10 MB
	logFileName = "service.log"
	oldFileName = "service.old.log"
)

// Rotate renames service.log -> service.old.log in the logs sub-directory of dataDir.
// Called on every service start.
func Rotate(dataDir string) error {
	dir := logDir(dataDir)
	logPath := filepath.Join(dir, logFileName)
	fi, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		return nil
	}
	oldPath := filepath.Join(dir, oldFileName)
	os.Remove(oldPath)
	return os.Rename(logPath, oldPath)
}

// Open returns an io.WriteCloser that appends to service.log under dataDir/logs/.
// It performs mid-session rotation when the file exceeds maxLogSize.
func Open(dataDir string) (io.WriteCloser, error) {
	dir := logDir(dataDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	path := filepath.Join(dir, logFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	return &rotatingWriter{path: path, dir: dir, f: f}, nil
}

func logDir(dataDir string) string {
	return filepath.Join(dataDir, "logs")
}

type rotatingWriter struct {
	mu   sync.Mutex
	path string
	dir  string
	f    *os.File
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.f.Write(p)
	if err != nil {
		return n, err
	}
	fi, statErr := w.f.Stat()
	if statErr == nil && fi.Size() > maxLogSize {
		w.f.Close()
		oldPath := filepath.Join(w.dir, oldFileName)
		os.Remove(oldPath)
		os.Rename(w.path, oldPath)
		newF, openErr := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if openErr == nil {
			w.f = newF
		}
	}
	return n, nil
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
