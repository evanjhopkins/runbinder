package platform

import (
	"errors"
	"os"
	"path/filepath"
)

const TaskLogName = ".runbinder.log"

type Paths struct {
	StorageDir  string
	Database    string
	InternalLog string
	ServiceLock string
}

func ResolvePaths() (Paths, error) {
	storageDir := os.Getenv("RUNBINDER_HOME")
	if storageDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, errors.New("resolve user home directory")
		}
		storageDir = filepath.Join(home, ".local", "share", "runbinder")
	}
	abs, err := filepath.Abs(storageDir)
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		StorageDir:  abs,
		Database:    filepath.Join(abs, "runbinder.db"),
		InternalLog: filepath.Join(abs, "runbinder.log"),
		ServiceLock: filepath.Join(abs, "service.lock"),
	}, nil
}

func EnsureStorage(paths Paths) error {
	return os.MkdirAll(paths.StorageDir, 0o700)
}
