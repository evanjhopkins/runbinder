package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPIDLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.pid")
	if _, err := ReadPID(path); !errors.Is(err, ErrNoPID) {
		t.Fatalf("missing PID error = %v", err)
	}
	if err := WritePID(path, 4242); err != nil {
		t.Fatal(err)
	}
	pid, err := ReadPID(path)
	if err != nil || pid != 4242 {
		t.Fatalf("PID = %d, error = %v", pid, err)
	}
	if err := RemovePID(path, 9999); err != nil {
		t.Fatal(err)
	}
	if pid, err := ReadPID(path); err != nil || pid != 4242 {
		t.Fatalf("mismatched removal changed PID: %d, %v", pid, err)
	}
	if err := RemovePID(path, 4242); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPID(path); !errors.Is(err, ErrNoPID) {
		t.Fatalf("removed PID error = %v", err)
	}
}

func TestRemovePIDClearsInvalidStaleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.pid")
	if err := os.WriteFile(path, []byte("not-a-pid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemovePID(path, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPID(path); !errors.Is(err, ErrNoPID) {
		t.Fatalf("stale PID error = %v", err)
	}
}
