package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const pidFileName = "eventrelay.pid"

// PIDFile manages a PID file for the eventrelay server.
type PIDFile struct {
	path string
}

// DefaultPIDPath returns the default PID file location.
func DefaultPIDPath() string {
	if dir := os.Getenv("EVENTRELAY_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, pidFileName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return pidFileName
	}
	return filepath.Join(home, ".config", "eventrelay", pidFileName)
}

// WritePIDFile creates a PID file. Returns an error if another instance is running.
func WritePIDFile(path string) (*PIDFile, error) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	// Check if an existing PID file points to a running process
	if data, err := os.ReadFile(path); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 1 {
			if pid, err := strconv.Atoi(parts[0]); err == nil {
				if processExists(pid) {
					return nil, fmt.Errorf("eventrelay already running (pid %d, pidfile %s)", pid, path)
				}
			}
		}
	}

	if err := os.WriteFile(path, fmt.Appendf(nil, "%d\n", os.Getpid()), 0644); err != nil {
		return nil, fmt.Errorf("writing pid file: %w", err)
	}
	return &PIDFile{path: path}, nil
}

// Remove cleans up the PID file.
func (p *PIDFile) Remove() {
	_ = os.Remove(p.path)
}

// ReadPIDFile reads the PID from a pid file and checks if the process is running.
// Returns pid, running, error.
func ReadPIDFile(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return 0, false, nil
	}
	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false, nil
	}
	return pid, processExists(pid), nil
}

// CheckPort checks if a port is in use.
func CheckPort(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500_000_000) // 500ms
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
