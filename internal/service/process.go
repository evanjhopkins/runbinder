package service

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var ErrNoPID = errors.New("service PID is not available")

func WritePID(path string, pid int) error {
	if pid <= 0 {
		return errors.New("service PID must be positive")
	}
	temporary := path + ".tmp." + strconv.Itoa(pid)
	if err := os.WriteFile(temporary, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write service PID: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish service PID: %w", err)
	}
	return nil
}

func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, ErrNoPID
	}
	if err != nil {
		return 0, fmt.Errorf("read service PID: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, errors.New("service PID file is invalid")
	}
	return pid, nil
}

func RemovePID(path string, expectedPID int) error {
	if expectedPID <= 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove service PID: %w", err)
		}
		return nil
	}
	pid, err := ReadPID(path)
	if errors.Is(err, ErrNoPID) {
		return nil
	}
	if err != nil {
		return err
	}
	if pid != expectedPID {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove service PID: %w", err)
	}
	return nil
}
